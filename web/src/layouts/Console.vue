<template>
  <t-layout class="layout">
    <t-header>
      <NavBar/>
    </t-header>
    <div class="content">
      <t-card class="list-card-container" :bordered="false">
        <router-view/>
      </t-card>
    </div>
    <t-footer>Sayrud</t-footer>
  </t-layout>
</template>

<script setup lang="ts">
import NavBar from "@/components/ConsoleNavBar.vue";
import {useAppStore} from "@/store";

const appStore = useAppStore()
if (appStore.theme === 'dark') {
  document.documentElement.setAttribute('theme-mode', 'dark');
}
</script>


<style scoped>
.layout {
  min-height: 100vh;
}

.content {
  flex-grow: 1;
  padding: 24px 24px 0;
}

.list-card-container {
  padding: var(--td-comp-paddingTB-xxl) var(--td-comp-paddingLR-xxl);

  :deep(.t-card__body) {
    padding: 0;
  }
}
</style>
