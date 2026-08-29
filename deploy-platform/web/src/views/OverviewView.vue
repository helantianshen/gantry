<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { Message } from '@arco-design/web-vue'
import { api, formatTime } from '../api'
import ReleaseRail from '../components/ReleaseRail.vue'
import StatusBadge from '../components/StatusBadge.vue'
import type { App, Deployment } from '../types'

const apps = ref<App[]>([])
const deployments = ref<Deployment[]>([])
const loading = ref(true)
const appNames = computed(() => Object.fromEntries(apps.value.map((app) => [app.id, app.name])))
const running = computed(() => deployments.value.filter((item) => ['pending', 'queued', 'running'].includes(item.status)))
const latest = computed(() => deployments.value[0])

onMounted(async () => {
  try {
    // ponytail: 单机应用超过 100 个后改为服务端搜索
    const [appPage, deploymentPage] = await Promise.all([api.apps(1, 100), api.deployments(1, 8)])
    apps.value = appPage.items
    deployments.value = deploymentPage.items
  } catch (error) {
    Message.error((error as Error).message)
  } finally {
    loading.value = false
  }
})
</script>

<template>
  <section class="page overview-page" :class="{ loading }" :aria-busy="loading">
    <div class="overview-intro">
      <div>
        <span class="eyebrow">SINGLE-HOST DELIVERY</span>
        <h2>让每一次容器切换<br />都有迹可循</h2>
      </div>
      <p>从 RabbitMQ 入队到健康检查、旧容器下线，Gantry 把单机发布收束为一条可观察的轨道</p>
    </div>

    <section class="live-board">
      <div class="section-heading">
        <div><span>LIVE TRACK</span><h3>{{ running.length ? `${running.length} 个任务正在轨道中` : '当前轨道空闲' }}</h3></div>
        <router-link to="/deployments">查看全部任务 →</router-link>
      </div>
      <template v-if="latest">
        <div class="latest-meta">
          <div><small>最近发布</small><strong>#{{ latest.id }} · {{ appNames[latest.app_id] || `应用 ${latest.app_id}` }}</strong></div>
          <StatusBadge :status="latest.status" />
        </div>
        <ReleaseRail :status="latest.status" />
      </template>
      <div v-else class="empty-track">还没有发布任务，从“应用与版本”创建第一条发布轨道</div>
    </section>

    <div class="overview-grid">
      <section class="panel recent-panel">
        <div class="section-heading"><div><span>RECENT RUNS</span><h3>最近发布</h3></div></div>
        <div v-if="deployments.length" class="run-list">
          <router-link v-for="item in deployments.slice(0, 5)" :key="item.id" :to="`/deployments/${item.id}`" class="run-row">
            <span class="run-id">#{{ item.id }}</span>
            <span><strong>{{ appNames[item.app_id] || `应用 ${item.app_id}` }}</strong><small>{{ formatTime(item.created_at) }}</small></span>
            <StatusBadge :status="item.status" />
          </router-link>
        </div>
        <a-empty v-else description="暂无发布记录" />
      </section>
      <section class="panel inventory-panel">
        <div class="section-heading"><div><span>INVENTORY</span><h3>应用清单</h3></div><b>{{ apps.length }}</b></div>
        <router-link v-for="app in apps.slice(0, 5)" :key="app.id" :to="`/apps/${app.id}`" class="inventory-row">
          <span>{{ app.name.slice(0, 2).toUpperCase() }}</span>
          <div><strong>{{ app.name }}</strong><small>{{ app.image_name }}</small></div>
          <i :class="{ active: app.current_version_id }" />
        </router-link>
        <a-empty v-if="!apps.length" description="还没有应用" />
      </section>
    </div>
  </section>
</template>
