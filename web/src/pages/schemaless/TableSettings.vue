<template>

</template>

<script setup lang="ts">
import {useRoute} from "vue-router";

const route = useRoute()
const projectUID = route.params.uid as string
const tableUID = route.params.tableUID as string
</script>

<style scoped>

</style>
