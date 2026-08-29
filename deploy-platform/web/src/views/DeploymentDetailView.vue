<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { Message } from '@arco-design/web-vue'
import { api, formatTime } from '../api'
import ReleaseRail from '../components/ReleaseRail.vue'
import StatusBadge from '../components/StatusBadge.vue'
import type { App, Deployment, DeploymentEvent, Version } from '../types'

const id = Number(useRoute().params.id)
const deployment = ref<Deployment>()
const events = ref<DeploymentEvent[]>([])
const app = ref<App>()
const version = ref<Version>()
const loading = ref(true)
const active = computed(() => deployment.value && ['pending', 'queued', 'running'].includes(deployment.value.status))
let timer: number | undefined

async function load(showLoading = false) {
  if (showLoading) loading.value = true
  try {
    const [dep, eventList] = await Promise.all([api.deployment(id), api.events(id)])
    deployment.value = dep
    events.value = eventList
    if (!app.value) {
      app.value = await api.app(dep.app_id)
      version.value = (await api.versions(dep.app_id)).find((item) => item.id === dep.version_id)
    }
    if (!active.value) window.clearInterval(timer)
  } catch (error) { Message.error((error as Error).message) }
  finally { loading.value = false }
}

onMounted(async () => {
  await load(true)
  if (active.value) timer = window.setInterval(() => load(), 2_000)
})
onBeforeUnmount(() => window.clearInterval(timer))
</script>

<template>
  <section class="page" :class="{ loading }" :aria-busy="loading">
    <router-link class="back-link" to="/deployments">← 返回发布任务</router-link>
    <div v-if="deployment" class="deployment-title">
      <div><span class="eyebrow">DEPLOYMENT #{{ deployment.id }}</span><h2>{{ app?.name || `应用 #${deployment.app_id}` }} <small>→</small> {{ version?.tag || `版本 #${deployment.version_id}` }}</h2><p>{{ formatTime(deployment.created_at) }} 创建 · 第 {{ deployment.attempt }} 次尝试</p></div>
      <StatusBadge :status="deployment.status" />
    </div>
    <section v-if="deployment" class="live-board detail-rail">
      <div class="section-heading"><div><span>RELEASE TRACK</span><h3>{{ active ? 'Worker 正在推进发布' : '本次发布已结束' }}</h3></div><small v-if="active" class="auto-refresh">每 2 秒自动刷新</small></div>
      <ReleaseRail :status="deployment.status" />
      <div v-if="deployment.fail_reason" class="failure-reason"><b>失败原因</b>{{ deployment.fail_reason }}</div>
    </section>
    <div v-if="deployment" class="detail-grid">
      <section class="panel event-panel">
        <div class="section-heading"><div><span>EVENT LOG</span><h3>事件时间线</h3></div><b>{{ events.length }}</b></div>
        <div v-if="events.length" class="event-list">
          <div v-for="event in events" :key="event.id" class="event-item">
            <i /><time>{{ formatTime(event.created_at) }}</time>
            <div><strong>{{ event.type }}</strong><p><span v-if="event.from_status">{{ event.from_status }} → </span>{{ event.to_status || '记录事件' }} · {{ event.actor }}</p></div>
          </div>
        </div>
        <a-empty v-else description="事件尚未写入" />
      </section>
      <aside class="panel facts-panel">
        <div class="section-heading"><div><span>RUNTIME FACTS</span><h3>执行信息</h3></div></div>
        <dl>
          <div><dt>消息 ID</dt><dd>{{ deployment.message_id }}</dd></div>
          <div><dt>来源版本</dt><dd>{{ deployment.from_version_id ? `#${deployment.from_version_id}` : '首次发布' }}</dd></div>
          <div><dt>目标版本</dt><dd>#{{ deployment.version_id }}</dd></div>
          <div><dt>执行 Worker</dt><dd>{{ deployment.lease_owner || '未取得执行权' }}</dd></div>
          <div><dt>Lease 到期</dt><dd>{{ formatTime(deployment.lease_expires_at) }}</dd></div>
        </dl>
      </aside>
    </div>
  </section>
</template>
