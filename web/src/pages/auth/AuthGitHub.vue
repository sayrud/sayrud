<template>
  <t-loading :loading="isLoading" text="登录中..." fullscreen/>
</template>

<script setup lang="ts">
import {onMounted, ref} from 'vue'
import {useRoute, useRouter} from "vue-router";
import {githubCallback} from "@/api/auth.ts";
import {useAppStore} from "@/store";

const isLoading = ref<boolean>(true)

const route = useRoute()
const router = useRouter()
const appStore = useAppStore()
const code = route.query.code ? route.query.code.toString() : ''

onMounted(() => {
  githubCallback(code).then(res => {
    const accessToken = res.token
    appStore.setToken(accessToken)

    router.push({name: 'Dashboard'})
  })
})
</script>

<style scoped>

</style>
