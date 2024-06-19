<template>
  <t-form
      ref="form"
      :class="['item-container']"
      label-width="0"
  >
    <t-form-item name="account">
      <t-input size="large">
        <template #prefix-icon>
          <t-icon name="user"/>
        </template>
      </t-input>
    </t-form-item>

    <t-form-item name="password">
      <t-input
          size="large"
          :type="showPassword ? 'text' : 'password'"
          clearable
      >
        <template #prefix-icon>
          <t-icon name="lock-on"/>
        </template>
        <template #suffix-icon>
          <t-icon :name="showPassword ? 'browse' : 'browse-off'" @click="showPassword = !showPassword"/>
        </template>
      </t-input>
    </t-form-item>

    <t-form-item class="btn-container">
      <t-button block size="large" type="submit"> 登录</t-button>
    </t-form-item>
  </t-form>
</template>

<script setup lang="ts">
import {ref} from 'vue'

const showPassword = ref<boolean>(false)
</script>

<style scoped>

</style>
