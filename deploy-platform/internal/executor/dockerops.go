package executor

import (
	"context"
	"fmt"
	"io"
	"log"
	"strconv"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
)

// DockerOps 使用 Docker SDK 实现发布所需的最小容器操作
type DockerOps struct {
	cli *client.Client
	// namespace 写入容器标签，避免多个 Gantry 实例互相发现和删除容器
	namespace string
}

// NewDockerOps 从环境创建 Docker 客户端，namespace 用于隔离不同 Gantry 实例的容器
func NewDockerOps(namespace string) (*DockerOps, error) {
	// FromEnv 读取 DOCKER_HOST 等标准环境变量
	// WithAPIVersionNegotiation 根据 daemon 版本选择双方兼容的 Docker API
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		// 客户端尚未创建成功，没有资源需要调用方清理
		return nil, fmt.Errorf("Docker 客户端初始化失败: %w", err)
	}
	return &DockerOps{
			cli:       cli,
			namespace: namespace,
		},
		nil
}

// Pull 确保 imageName:tag 已存在，本地命中时不重复拉取
func (d *DockerOps) Pull(ctx context.Context, imageName, tag string) error {
	// Docker SDK 使用完整镜像引用，例如 nginx:1.27
	ref := fmt.Sprintf("%s:%s", imageName, tag)

	// ImageInspectWithRaw 只查询本地镜像元数据，成功说明镜像已经可用
	if _, _, err := d.cli.ImageInspectWithRaw(ctx, ref); err == nil {
		return nil
	}

	// 本地查询失败后请求 daemon 拉取镜像，reader 持续输出拉取进度
	reader, err := d.cli.ImagePull(ctx, ref, image.PullOptions{})
	if err != nil {
		// 请求未建立成功，没有响应流需要处理
		return fmt.Errorf("拉取镜像失败 %s: %w", ref, err)
	}
	defer reader.Close()
	// 必须消费完整响应流，Docker daemon 才能完成并报告整个拉取过程
	// 当前不展示进度，因此直接丢弃流内容
	_, _ = io.Copy(io.Discard, reader)
	return nil
}

// Stop 在 10 秒宽限期内停止容器，空 ID 直接成功
func (d *DockerOps) Stop(ctx context.Context, containerID string) error {
	if containerID == "" {
		// 上层没有找到旧容器时无需调用 Docker
		return nil
	}
	// Docker 先向容器主进程发送停止信号，10 秒后仍未退出则强制终止
	timeout := 10
	if err := d.cli.ContainerStop(ctx, containerID, container.StopOptions{Timeout: &timeout}); err != nil {
		return fmt.Errorf("停止容器失败 %s: %w", shortID(containerID), err)
	}
	return nil
}

// Remove 强制删除容器，空 ID 直接成功
func (d *DockerOps) Remove(ctx context.Context, containerID string) error {
	if containerID == "" {
		// 空 ID 表示当前流程没有需要清理的容器
		return nil
	}
	// Force 允许直接删除仍在运行或未正常停止的容器
	if err := d.cli.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: true}); err != nil {
		return fmt.Errorf("删除容器失败 %s: %w", shortID(containerID), err)
	}
	return nil
}

// Run 清理同任务遗留容器后启动新容器
// 返回值依次为新容器 ID、宿主机健康检查地址和错误，启动后检查失败时仍返回容器 ID 供上层清理
func (d *DockerOps) Run(ctx context.Context, appID, deploymentID int64, imageName, tag string) (containerID string, containerIP string, err error) {
	ref := fmt.Sprintf("%s:%s", imageName, tag)

	// 两个 label 筛选条件把范围限制在当前 Gantry 实例的本次发布任务
	f := filters.NewArgs()
	f.Add("label", "gantry.instance="+d.namespace)
	f.Add("label", fmt.Sprintf("deployment-id=%d", deploymentID))
	// All=true 同时查询运行中和已停止的容器，确保重试前不遗漏失败残留
	stale, err := d.cli.ContainerList(ctx, container.ListOptions{All: true, Filters: f})
	if err != nil {
		// 尚未创建新容器，因此三个返回值都为空
		return "", "", fmt.Errorf("查找任务 %d 遗留容器失败: %w", deploymentID, err)
	}
	for i := range stale {
		// 同一 deployment 重试前清理自己遗留的容器，不触碰其他任务
		if err := d.cli.ContainerRemove(ctx, stale[i].ID, container.RemoveOptions{Force: true}); err != nil {
			// 遗留容器未清理干净时停止发布，避免同一任务产生多个实例
			return "", "", fmt.Errorf("清理任务 %d 遗留容器失败: %w", deploymentID, err)
		}
	}

	// ContainerCreate 只创建容器配置，此时容器还没有启动
	// Image 指定待运行镜像，Labels 供重试清理和旧容器查找使用
	resp, err := d.cli.ContainerCreate(ctx, &container.Config{
		Image: ref,
		Labels: map[string]string{
			"gantry.instance": d.namespace,
			"app":             strconv.FormatInt(appID, 10),
			"deployment-id":   strconv.FormatInt(deploymentID, 10),
		},
	}, &container.HostConfig{
		// Docker daemon 或宿主机重启后自动恢复容器，主动 Stop 除外
		RestartPolicy: container.RestartPolicy{
			Name: "unless-stopped",
		},
		// Docker Desktop on WSL2 无法稳定直连容器 bridge IP
		// HostPort 留空让 Docker 分配本机随机端口，并且只绑定到 127.0.0.1
		PortBindings: nat.PortMap{
			"80/tcp": []nat.PortBinding{{HostIP: "127.0.0.1", HostPort: ""}},
		},
	}, nil, nil, "")
	if err != nil {
		// 创建失败时 Docker 没有返回可供上层清理的容器 ID
		return "", "", fmt.Errorf("创建任务 %d 容器失败: %w", deploymentID, err)
	}

	// ContainerStart 才真正启动上一步创建的容器
	if err := d.cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		// 启动失败时尽力删除刚创建的容器，避免留下不可运行实例
		_ = d.cli.ContainerRemove(ctx, resp.ID, container.RemoveOptions{Force: true})
		// 已执行本地清理，因此不再把容器 ID 交给上层重复处理
		return "", "", fmt.Errorf("启动任务 %d 容器失败: %w", deploymentID, err)
	}

	// 启动后重新查询容器详情，获取 Docker 实际分配的宿主端口
	inspect, err := d.cli.ContainerInspect(ctx, resp.ID)
	if err != nil {
		// 容器已经启动，返回 ID 让上层在失败收口时将其删除
		return resp.ID, "", fmt.Errorf("检查任务 %d 容器失败: %w", deploymentID, err)
	}
	addr := ""
	if netSet := inspect.NetworkSettings; netSet != nil {
		// Ports 记录容器端口到宿主端口的实际绑定结果
		for port, bindings := range netSet.Ports {
			if port == "80/tcp" && len(bindings) > 0 && bindings[0].HostPort != "" {
				addr = fmt.Sprintf("127.0.0.1:%s", bindings[0].HostPort)
				break
			}
		}
	}
	if addr == "" {
		// 容器已启动但无法健康检查，仍返回 ID 供上层补偿删除
		return resp.ID, "", fmt.Errorf("任务 %d 容器未获得宿主端口", deploymentID)
	}

	log.Printf("容器已启动: %s (image=%s, deployment=%d, addr=%s)", resp.ID[:12], ref, deploymentID, addr)
	// 成功时同时返回容器 ID 和健康检查使用的宿主地址
	return resp.ID, addr, nil
}

// Inspect 查找 appID 当前唯一的旧稳定容器，并排除本次 deployment 创建的新容器
// 没有旧容器时返回空 ID，发现多个稳定容器时返回错误阻止不确定切换
func (d *DockerOps) Inspect(ctx context.Context, appID, deploymentID int64) (string, error) {
	// namespace 隔离 Gantry 实例，app 标签把范围缩小到当前应用
	f := filters.NewArgs()
	f.Add("label", "gantry.instance="+d.namespace)
	f.Add("label", fmt.Sprintf("app=%d", appID))
	// 只查找正在运行的容器，已停止的历史实例不参与新旧版本切换
	f.Add("status", "running")
	containers, err := d.cli.ContainerList(ctx, container.ListOptions{
		Filters: f,
	})
	if err != nil {
		// 查询失败时无法安全判断哪个容器是旧版本，因此阻止后续切换
		return "", fmt.Errorf("查找容器失败: %w", err)
	}

	// 复用查询结果的底层数组保存旧稳定容器，避免额外分配
	stable := containers[:0]
	currentDeployment := strconv.FormatInt(deploymentID, 10)
	for i := range containers {
		// 排除本次发布刚创建的新容器，剩余项才是待替换的旧容器
		if containers[i].Labels["deployment-id"] != currentDeployment {
			stable = append(stable, containers[i])
		}
	}
	if len(stable) == 0 {
		// 首次发布没有旧容器需要停止，空 ID 配合 nil 表示正常情况
		return "", nil
	}
	if len(stable) > 1 {
		// 无法确定应该替换哪一个时拒绝随机选择，交由人工清理异常实例
		return "", fmt.Errorf("应用 %d 存在 %d 个稳定容器，请先清理残留实例", appID, len(stable))
	}
	// 恰好一个旧容器时返回其完整 ID，供上层在新容器健康后停止和删除
	return stable[0].ID, nil
}

// shortID 截取 Docker 日志中便于识别的 12 位容器 ID
func shortID(id string) string {
	if len(id) > 12 {
		return id[:12]
	}
	return id
}
