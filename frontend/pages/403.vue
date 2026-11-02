<template>
  <a-result
    status="403"
    title="403"
    sub-title="Xin lỗi, bạn không có quyền truy cập vào chức năng hoặc trang này."
  >
    <template #extra>
      <a-button type="primary" @click="navigateTo('/dashboard')">
        Quay về Trang chủ
      </a-button>
    </template>
  </a-result>
</template>

<script setup lang="ts">
// Trang 403 Forbidden
</script>
