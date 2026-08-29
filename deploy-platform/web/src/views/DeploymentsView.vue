<script setup lang="ts">
import { onMounted, ref, watch } from 'vue'
import { Message } from '@arco-design/web-vue'
import { api, formatTime } from '../api'
import StatusBadge from '../components/StatusBadge.vue'
import { deploymentStatusLabels, type App, type Deployment, type DeploymentStatus } from '../types'

const deployments = ref<Deployment[]>([])
const apps = ref<App[]>([])
const total = ref(0)
const page = ref(1)
const appID = ref<number>()
const status = ref<string>()
const loading = ref(false)
const statuses = Object.entries(deploymentStatusLabels) as [DeploymentStatus, string][]

function appName(id: number) { return apps.value.find((app) => app.id === id)?.name || `应用 #${id}` }

async function load() {
  loading.value = true
  try {
    const result = await api.deployments(page.value, 20, appID.value, status.value)
    deployments.value = result.items
    total.value = result.total
  } catch (error) { Message.error((error as Error).message) }
  finally { loading.value = false }
}

onMounted(async () => {
  // ponytail: 单机应用超过 100 个后改为服务端搜索
  api.apps(1, 100).then((result) => { apps.value = result.items }).catch(() => undefined)
  await load()
})
watch([appID, status], () => { page.value = 1; load() })
</script>

<template>
  <section class="page">
    <div class="page-lead">
      <div><span class="eyebrow">DEPLOYMENT LEDGER</span><h2>发布任务</h2><p>每条记录对应一次完整的状态机流转，失败任务会保留回滚结果与原因</p></div>
    </div>
    <div class="filter-bar">
      <a-select v-model="appID" allow-clear placeholder="全部应用" :style="{ width: '220px' }">
        <a-option v-for="app in apps" :key="app.id" :value="app.id">{{ app.name }}</a-option>
      </a-select>
      <a-select v-model="status" allow-clear placeholder="全部状态" :style="{ width: '180px' }">
        <a-option v-for="item in statuses" :key="item[0]" :value="item[0]">{{ item[1] }}</a-option>
      </a-select>
      <span>{{ total }} 条记录</span>
    </div>
    <div class="table-shell" :class="{ loading }" :aria-busy="loading">
      <table>
        <thead><tr><th>任务</th><th>应用</th><th>版本</th><th>状态</th><th>尝试</th><th>创建时间</th><th /></tr></thead>
        <tbody>
          <tr v-for="item in deployments" :key="item.id">
            <td><strong>#{{ item.id }}</strong><small>{{ item.message_id.slice(0, 8) }}</small></td>
            <td>{{ appName(item.app_id) }}</td>
            <td>#{{ item.version_id }}</td>
            <td><StatusBadge :status="item.status" /></td>
            <td>{{ item.attempt }}</td>
            <td>{{ formatTime(item.created_at) }}</td>
            <td><router-link :to="`/deployments/${item.id}`">查看详情 →</router-link></td>
          </tr>
        </tbody>
      </table>
      <a-empty v-if="!deployments.length && !loading" description="没有符合条件的发布任务" />
    </div>
    <a-pagination v-if="total > 20" v-model:current="page" :total="total" :page-size="20" @change="load" />
  </section>
</template>
