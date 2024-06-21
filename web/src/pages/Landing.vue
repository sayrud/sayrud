<template>
  <div class="container">
    <div class="banner">
      <div class="banner-content">
        <div class="banner-title">
          <img src="@/assets/logo.svg"/>
        </div>
        <div class="banner-subtitle">我不想写 CRUD 了</div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">

</script>

<style scoped>
.container {
  display: flex;
  height: 100vh;
  width: 100vw;
  flex-direction: column;
  justify-content: center;
  align-items: center;
}

.banner {
  width: 100%;
  position: relative;
  background-image: "";
  background-size: auto 720px;
  background-position: top 0px right 0;
  background-repeat: no-repeat;
}

.banner-content {
  max-width: 1440px;
  padding: 48px 96px;
  box-sizing: border-box;
  margin: 0 auto;
}

.banner-title {
  font-size: 64px;
  font-family: "Noto Sans SC", sans-serif;
  line-height: 75px;
  color: #000;
  letter-spacing: -1px;
}

.banner-subtitle {
  font-style: normal;
  font-family: "Noto Sans SC", sans-serif;
  font-weight: 400;
  font-size: 24px;
  line-height: 44px;
  margin-top: 8px;
  color: rgba(0, 0, 0, .6);
  margin-bottom: 48px;
  letter-spacing: -1px;
}
</style>
