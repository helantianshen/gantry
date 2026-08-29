#!/usr/bin/env bash
# 实验 03：OverlayFS 联合挂载
# 覆盖：lower/upper/work/merged + CoW + whiteout
# 预期：exit 0，断言 CoW 行为正确（修改不影响 lower）

set -euo pipefail
source "$(dirname "$0")/lib.sh"

readonly LAB_NAME="lab-overlayfs"
readonly WORKDIR="/tmp/${LAB_NAME}"

cleanup() {
	log "清理 ${LAB_NAME}"
	[ -d "$WORKDIR/merged" ] && umount "$WORKDIR/merged" 2>/dev/null || true
	[ -d "$WORKDIR" ] && rm -rf "$WORKDIR" 2>/dev/null || true
}
trap cleanup EXIT

# 主流程
log "=== 实验 03：OverlayFS 联合挂载 ==="

# 1. 准备目录结构
mkdir -p "$WORKDIR"/{lower,upper,work,merged}
log "创建目录：lower upper work merged"

# 2. 在 lower 中放一个文件（模拟只读基础层）
echo "lower-content" > "$WORKDIR/lower/base.txt"
echo "shared-readonly" > "$WORKDIR/lower/shared.txt"
log "lower 层写入 base.txt + shared.txt"

# 3. 挂载 OverlayFS
mount -t overlay overlay \
	-o "lowerdir=$WORKDIR/lower,upperdir=$WORKDIR/upper,workdir=$WORKDIR/work" \
	"$WORKDIR/merged"
log "OverlayFS 挂载到 merged"

# 4. 断言：merged 中能看到 lower 的文件
MERGED_BASE=$(cat "$WORKDIR/merged/base.txt" 2>/dev/null || echo "NOT_FOUND")
assert_equals "$MERGED_BASE" "lower-content"

# 5. CoW 测试：修改 merged/shared.txt，验证 lower/shared.txt 不变
echo "modified-in-upper" > "$WORKDIR/merged/shared.txt"
LOWER_SHARED=$(cat "$WORKDIR/lower/shared.txt")
MERGED_SHARED=$(cat "$WORKDIR/merged/shared.txt")
assert_equals "$LOWER_SHARED" "shared-readonly"
assert_equals "$MERGED_SHARED" "modified-in-upper"
log "CoW 验证通过：lower 不变，修改写入 upper"

# 6. whiteout 测试：删除 merged/base.txt
rm "$WORKDIR/merged/base.txt"
if [ -f "$WORKDIR/lower/base.txt" ]; then
	log "lower/base.txt 仍在（whiteout 仅影响 merged 视图）"
else
	log "断言失败：lower/base.txt 被误删"
	exit 1
fi
if [ -f "$WORKDIR/merged/base.txt" ]; then
	log "断言失败：merged/base.txt 仍存在"
	exit 1
fi
log "whiteout 验证通过：merged 中 base.txt 不可见，lower 中未删除"

log "=== 实验 03 完成：OverlayFS CoW + whiteout 验证通过 ==="
