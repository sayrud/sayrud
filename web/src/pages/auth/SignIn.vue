<template>
  <div class="login-wrapper">
    <header class="login-header">
      <img alt="logo" class="logo" :src="Logo"/>
    </header>

    <div class="login-container">
      <div class="title-container">
        <h1 class="title">登录 Sayrud</h1>
      </div>
      <t-form ref="form" class="item-container" label-width="0">
        <t-form-item class="btn-container">
          <t-button block size="large" type="submit" @click="onSignIn">
            <template #icon>
              <LogoGithubIcon/>
            </template>
            GitHub 登录
          </t-button>
        </t-form-item>
      </t-form>
    </div>

    <footer class="copyright">Copyright @ 2024 Sayrud. All Rights Reserved</footer>
  </div>
</template>

<script setup lang="ts">
import Logo from '@/assets/logo.svg';
import {LogoGithubIcon} from 'tdesign-icons-vue-next';

const onSignIn = () => {
  window.location.href = `https://github.com/login/oauth/authorize?client_id=${import.meta.env.VITE_GITHUB_CLIENT_ID}&redirect_uri=${import.meta.env.VITE_GITHUB_REDIRECT_URI}&scope=user`;
}
</script>

<style lang="less" scoped>
.login-header {
  padding: 0 var(--td-comp-paddingLR-xl);
  display: flex;
  justify-content: space-between;
  align-items: center;
  backdrop-filter: blur(10px);
  color: var(--td-text-color-primary);
  height: var(--td-comp-size-xxxl);

  .logo {
    height: var(--td-comp-size-l);
  }
}

.login-wrapper {
  background-color: white;
  background-image: url('@/assets/assets-signin-bg.png');
}

.login-wrapper {
  height: 100vh;
  display: flex;
  flex-direction: column;
  background-size: cover;
  background-position: 100%;
  position: relative;
}

.login-container {
  position: absolute;
  top: 22%;
  left: 5%;
  min-height: 500px;
}

.title-container {
  .title {
    font: var(--td-font-headline-large);
    color: var(--td-text-color-primary);
    margin-top: var(--td-comp-margin-xs);

    &.margin-no {
      margin-top: 0;
    }
  }
}

.item-container {
  width: 400px;
  margin-top: var(--td-comp-margin-xxxxl);

  &.login-qrcode {
    .tip-container {
      margin-bottom: var(--td-comp-margin-l);
      font: var(--td-font-body-medium);
      display: flex;
      align-items: flex-start;

      .tip {
        color: var(--td-text-color-primary);
        margin-right: var(--td-comp-margin-s);
      }

      .refresh {
        display: flex;
        align-items: center;
        color: var(--td-brand-color);

        .t-icon {
          font-size: var(--td-comp-size-xxxs);
          margin-left: var(--td-comp-margin-xs);
        }

        &:hover {
          cursor: pointer;
        }
      }
    }

    .bottom-container {
      margin-top: 32px;
    }
  }

  &.login-phone {
    .bottom-container {
      margin-top: 66px;
    }
  }

  .check-container {
    display: flex;
    align-items: center;

    &.remember-pwd {
      margin-bottom: var(--td-comp-margin-l);
      justify-content: space-between;
    }

    span {
      color: var(--td-brand-color);

      &:hover {
        cursor: pointer;
      }
    }
  }

  .verification-code {
    display: flex;
    align-items: center;

    :deep(.t-form__controls) {
      width: 100%;

      button {
        flex-shrink: 0;
        margin-left: var(--td-comp-margin-l);
        width: 128px;
      }
    }
  }

  .btn-container {
    margin-top: var(--td-comp-margin-xxxxl);
  }
}

.switch-container {
  margin-top: var(--td-comp-margin-xxl);

  .tip {
    font: var(--td-font-body-medium);
    color: var(--td-brand-color);
    cursor: pointer;
    display: inline-flex;
    align-items: center;
    margin-right: var(--td-comp-margin-l);

    &:last-child {
      &::after {
        display: none;
      }
    }

    &::after {
      content: '';
      display: block;
      width: 1px;
      height: 12px;
      background: var(--td-component-stroke);
      margin-left: var(--td-comp-margin-l);
    }
  }
}

.check-container {
  font: var(--td-font-body-medium);
  color: var(--td-text-color-secondary);

  .tip {
    float: right;
    font: var(--td-font-body-medium);
    color: var(--td-brand-color);
  }
}

.copyright {
  font: var(--td-font-body-medium);
  position: absolute;
  left: 5%;
  bottom: 64px;
  color: var(--td-text-color-secondary);
}

@media screen and (height <= 700px) {
  .copyright {
    display: none;
  }
}
</style>
