<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { Message } from '@arco-design/web-vue'
import { api, formatTime } from '../api'
import type { App } from '../types'

const apps = ref<App[]>([])
const total = ref(0)
const page = ref(1)
const loading = ref(false)
const visible = ref(false)
const saving = ref(false)
const form = reactive({ name: '', image_name: '', healthcheck_path: '/healthz', healthcheck_timeout_sec: 60 })

async function load() {
  loading.value = true
  try {
    const result = await api.apps(page.value, 12)
    apps.value = result.items
    total.value = result.total
  } catch (error) { Message.error((error as Error).message) }
  finally { loading.value = false }
}

async function create() {
  saving.value = true
  try {
    await api.createApp(form)
    Message.success('应用已创建')
    visible.value = false
    Object.assign(form, { name: '', image_name: '', healthcheck_path: '/healthz', healthcheck_timeout_sec: 60 })
    await load()
  } catch (error) { Message.error((error as Error).message) }
  finally { saving.value = false }
}

onMounted(load)
</script>

<template>
  <section class="page">
    <div class="page-lead">
      <div><span class="eyebrow">APPLICATION REGISTRY</span><h2>应用与版本</h2><p>应用定义镜像仓库和健康检查策略，版本记录每次可发布的镜像标签</p></div>
      <a-button type="primary" size="large" @click="visible = true">新建应用</a-button>
    </div>
    <div class="app-grid" :class="{ loading }" :aria-busy="loading">
      <router-link v-for="app in apps" :key="app.id" :to="`/apps/${app.id}`" class="app-card">
        <div class="app-card-top"><span>{{ app.name.slice(0, 2).toUpperCase() }}</span><small>#{{ app.id }}</small></div>
        <h3>{{ app.name }}</h3>
        <code>{{ app.image_name }}</code>
        <dl><div><dt>当前版本</dt><dd>{{ app.current_version_id ? `#${app.current_version_id}` : '未发布' }}</dd></div><div><dt>健康检查</dt><dd>{{ app.healthcheck_path }}</dd></div></dl>
        <footer>更新于 {{ formatTime(app.updated_at) }} <b>→</b></footer>
      </router-link>
      <button v-if="!apps.length && !loading" class="create-empty" @click="visible = true"><b>＋</b><span>创建第一个应用</span></button>
    </div>
    <a-pagination v-if="total > 12" v-model:current="page" :total="total" :page-size="12" @change="load" />

    <a-modal v-model:visible="visible" title="新建应用" :ok-loading="saving" ok-text="创建应用" @ok="create">
      <a-form :model="form" layout="vertical">
        <a-form-item label="应用名称" required><a-input v-model="form.name" placeholder="例如 order-api" /></a-form-item>
        <a-form-item label="镜像名称" required><a-input v-model="form.image_name" placeholder="例如 registry.local/order-api" /></a-form-item>
        <div class="form-pair">
          <a-form-item label="健康检查路径"><a-input v-model="form.healthcheck_path" /></a-form-item>
          <a-form-item label="超时（秒）"><a-input-number v-model="form.healthcheck_timeout_sec" :min="1" :max="300" /></a-form-item>
        </div>
      </a-form>
    </a-modal>
  </section>
</template>
