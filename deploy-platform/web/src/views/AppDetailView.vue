<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Message, Modal } from '@arco-design/web-vue'
import { api, formatTime } from '../api'
import type { App, Version } from '../types'

const route = useRoute()
const router = useRouter()
const appID = Number(route.params.id)
const app = ref<App>()
const versions = ref<Version[]>([])
const loading = ref(true)
const visible = ref(false)
const saving = ref(false)
const form = reactive({ tag: '', description: '' })

async function load() {
  loading.value = true
  try { [app.value, versions.value] = await Promise.all([api.app(appID), api.versions(appID)]) }
  catch (error) { Message.error((error as Error).message) }
  finally { loading.value = false }
}

async function createVersion() {
  saving.value = true
  try {
    await api.createVersion(appID, form)
    Message.success('版本已创建')
    visible.value = false
    Object.assign(form, { tag: '', description: '' })
    await load()
  } catch (error) { Message.error((error as Error).message) }
  finally { saving.value = false }
}

function deploy(version: Version) {
  Modal.confirm({
    title: `发布 ${version.tag}`,
    content: `将 ${app.value?.name} 切换到镜像标签 ${version.tag}，任务创建后由 Worker 异步执行`,
    okText: '创建发布',
    async onOk() {
      try {
        const result = await api.deploy(appID, version.id)
        Message.success('发布任务已进入队列')
        router.push(`/deployments/${result.id}`)
      } catch (error) { Message.error((error as Error).message) }
    },
  })
}

onMounted(load)
</script>

<template>
  <section class="page" :class="{ loading }" :aria-busy="loading">
    <router-link class="back-link" to="/apps">← 返回应用列表</router-link>
    <div v-if="app" class="app-detail-head">
      <div class="app-monogram">{{ app.name.slice(0, 2).toUpperCase() }}</div>
      <div><span class="eyebrow">APP #{{ app.id }}</span><h2>{{ app.name }}</h2><code>{{ app.image_name }}</code></div>
      <div class="health-policy"><small>健康检查策略</small><strong>{{ app.healthcheck_path }}</strong><span>{{ app.healthcheck_timeout_sec }} 秒超时</span></div>
    </div>
    <section class="panel version-panel">
      <div class="section-heading"><div><span>VERSION HISTORY</span><h3>版本历史</h3></div><a-button type="primary" @click="visible = true">登记版本</a-button></div>
      <div v-if="versions.length" class="version-list">
        <div v-for="version in versions" :key="version.id" class="version-row">
          <span class="version-tag">{{ version.tag }}</span>
          <div><strong>{{ version.description || '未填写版本说明' }}</strong><small>登记于 {{ formatTime(version.created_at) }} · ID #{{ version.id }}</small></div>
          <span v-if="app?.current_version_id === version.id" class="current-mark">当前运行</span>
          <a-button v-else @click="deploy(version)">发布此版本</a-button>
        </div>
      </div>
      <a-empty v-else description="还没有版本，请先登记一个镜像标签" />
    </section>

    <a-modal v-model:visible="visible" title="登记新版本" :ok-loading="saving" ok-text="登记版本" @ok="createVersion">
      <a-form :model="form" layout="vertical">
        <a-form-item label="镜像标签" required><a-input v-model="form.tag" placeholder="例如 v1.4.0" /></a-form-item>
        <a-form-item label="版本说明"><a-textarea v-model="form.description" placeholder="本次版本解决了什么问题" :max-length="200" show-word-limit /></a-form-item>
      </a-form>
    </a-modal>
  </section>
</template>
