<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { api } from './api'

const route = useRoute()
const healthy = ref(false)
const title = computed(() => String(route.meta.title || 'Gantry'))
let timer: number | undefined

async function checkHealth() {
  healthy.value = await api.health().then(() => true).catch(() => false)
}

onMounted(() => {
  checkHealth()
  timer = window.setInterval(checkHealth, 10_000)
})
onBeforeUnmount(() => window.clearInterval(timer))
</script>

<template>
  <div class="app-shell">
    <aside class="sidebar">
      <router-link class="brand" to="/" aria-label="Gantry 首页">
        <span class="brand-mark" aria-hidden="true"><i /></span>
        <span><b>GANTRY</b><small>RELEASE CONTROL</small></span>
      </router-link>
      <nav aria-label="主导航">
        <router-link to="/" exact-active-class="active"><span>01</span>发布总览</router-link>
        <router-link to="/apps" active-class="active"><span>02</span>应用与版本</router-link>
        <router-link to="/deployments" active-class="active"><span>03</span>发布任务</router-link>
      </nav>
      <div class="system-state">
        <i :class="{ online: healthy }" />
        <span><small>API 状态</small>{{ healthy ? '运行正常' : '连接中断' }}</span>
      </div>
    </aside>

    <main>
      <header class="topbar">
        <div><small>CONTROL ROOM</small><h1>{{ title }}</h1></div>
        <router-link class="new-release" to="/apps">选择版本并发布 <span>↗</span></router-link>
      </header>
      <router-view />
    </main>
  </div>
</template>
