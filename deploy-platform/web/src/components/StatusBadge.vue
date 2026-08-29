<script setup lang="ts">
import { deploymentStatusLabels, type DeploymentStatus } from '../types'

defineProps<{ status: DeploymentStatus }>()
</script>

<template>
  <span class="status-badge" :class="`status-${status}`">
    <i aria-hidden="true" />{{ deploymentStatusLabels[status] }}
  </span>
</template>
