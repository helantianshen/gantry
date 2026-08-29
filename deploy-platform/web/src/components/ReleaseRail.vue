<script setup lang="ts">
import { computed } from 'vue'
import type { DeploymentStatus } from '../types'

const props = defineProps<{ status: DeploymentStatus }>()
const stages = ['创建任务', '进入队列', '切换容器', '完成']
const progress = computed(() => {
  const positions: Partial<Record<DeploymentStatus, number>> = { pending: 0, queued: 1, running: 2, success: 3 }
  return positions[props.status] ?? 2
})
const failed = computed(() => ['failed', 'rolled_back', 'failed_rollback'].includes(props.status))
</script>

<template>
  <div class="release-rail" :class="{ 'is-failed': failed }" :aria-label="`当前发布状态：${status}`">
    <div v-for="(stage, index) in stages" :key="stage" class="rail-stage" :class="{ active: index <= progress, current: index === progress }">
      <span class="rail-node">{{ String(index + 1).padStart(2, '0') }}</span>
      <span class="rail-label">{{ index === 3 && failed ? '异常收口' : stage }}</span>
    </div>
  </div>
</template>
