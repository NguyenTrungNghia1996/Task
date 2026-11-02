<template>
  <div style="text-align: center; padding: 50px;">
    <a-spin size="large" />
  </div>
</template>

<script setup lang="ts">
import { onMounted } from 'vue'
import { useRouter } from 'vue-router'

const router = useRouter()
onMounted(() => {
  router.replace('/dashboard')
})
</script>
