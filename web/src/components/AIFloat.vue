<template>
  <div class="floating">
    <Transition name="popup">
      <t-card v-if="showDialog" bordered header-bordered class="dialog-card" title="AI 建议">
        <template #actions>
          <t-button @click="onClean" variant="text" theme="primary">清空</t-button>
        </template>

        <div ref="list" class="body">
          <t-list :split="true">
            <t-list-item v-for="(message, index) in dialogMessages" :key="index">
              <template #content>
                <t-comment :author="message.role" :content="message.content">
                  <template #author>
                    {{ message.role === 'user' ? '我' : 'AI' }}
                  </template>
                  <template #content>
                    <div v-if="message.role === 'user'">
                      {{ message.content }}
                    </div>
                    <div v-else>
                      <div v-if="message.action">
                        <div class="action">
                          <div v-if="message.action === 'tables'">
                            <t-base-table size="small" row-key="tableName" :data="message.actionJson" :columns="[
                            {title: '表名', colKey: 'tableName'},
                            {title: '备注', colKey: 'tableLabel'}
                        ]"></t-base-table>
                          </div>
                          <t-button @click="onApply(message.action, message.actionJson)" size="small" theme="primary"
                                    variant="outline">应用
                          </t-button>
                        </div>
                        <t-divider dashed/>
                        <div v-html="renderMarkdown(message.content)"></div>
                      </div>
                      <div v-else v-html="renderMarkdown(message.raw)"></div>
                    </div>
                  </template>
                </t-comment>
              </template>
            </t-list-item>
            <t-list-item v-if="dialogResponseLoading">
              <t-loading text="加载中..." size="small"></t-loading>
            </t-list-item>
          </t-list>
        </div>

        <template #footer>
          <t-comment>
            <template #content>
              <t-input size="large" v-model="dialogInput" placeholder="说点什么吧？" @enter="onSend"/>
            </template>
          </t-comment>
        </template>
      </t-card>
    </Transition>

    <t-button size="large" shape="circle" theme="primary" class="btn" @click="showDialog = !showDialog">
      <template #icon>
        <ChatBubbleSmileIcon size="24px"/>
      </template>
    </t-button>
  </div>
</template>

<script setup lang="ts">
import {nextTick, ref} from 'vue'
import {ChatBubbleSmileIcon} from 'tdesign-icons-vue-next';
import {aiAdvice, aiApply} from '@/api/ai.ts';
import {useRoute} from "vue-router";
import {Marked} from 'marked';
import DOMPurify from 'dompurify';
import {MessagePlugin} from "tdesign-vue-next";

const route = useRoute()
const projectUID = route.params.uid as string
const emits = defineEmits(['refresh'])
const list = ref()

const dialogMessages = ref<{
  raw: string,
  role: 'user' | 'assistant',
  content: string,
  action?: string,
  actionJson?: any,
}[]>([])
const dialogInput = ref<string>('')
const dialogResponseLoading = ref<boolean>(false)
const showDialog = ref<boolean>(false)

const onClean = () => {
  dialogMessages.value = []
}

const onSend = () => {
  if (dialogInput.value === '') {
    return
  }

  dialogMessages.value.push({
    raw: dialogInput.value,
    role: 'user',
    content: dialogInput.value,
  })
  dialogInput.value = ''

  dialogResponseLoading.value = true

  const messages = dialogMessages.value.map((message => {
    return {
      role: message.role,
      content: message.raw,
    }
  }))

  nextTick(() => {
    list.value.scrollTop = list.value.scrollHeight;
  })

  aiAdvice(projectUID, {
    action: 'tables',
    messages: messages,
  }).then(res => {
    dialogMessages.value.push({
      raw: res.raw,
      role: 'assistant',
      content: res.description,
      action: res.action,
      actionJson: res.actionJson,
    })
  }).finally(() => {
    dialogResponseLoading.value = false
    nextTick(() => {
      list.value.scrollTop = list.value.scrollHeight;
    })
  })
}

const onApply = (action: string, actionJson: any) => {
  aiApply(projectUID, {
    action: action,
    actionJson: actionJson,
  }).then(() => {
    MessagePlugin.success('应用成功')
  }).finally(() => {
    emits('refresh')
  })
}

const renderMarkdown = (content: string) => {
  const marked = new Marked();
  marked.setOptions({breaks: true});
  return DOMPurify.sanitize(marked.parse(content).toString()) as string;
}
</script>

<style scoped lang="less">
.floating {
  position: fixed;
  right: 50px;
  bottom: 50px;
  z-index: 1000;

  .btn {
    width: 60px;
    height: 60px;
    border-radius: 60px;
  }

  .dialog-card {
    position: absolute;
    right: 0;
    bottom: 80px;
    width: 400px;
  }

  :deep(.t-card__body) {
    padding: 0;
  }
}

.body {
  max-height: 600px;
  overflow-y: auto;
}

.action {
  display: flex;
  flex-direction: column;
  gap: 8px;
  justify-content: space-between;
  align-items: normal;
}

.popup-enter-active,
.popup-leave-active {
  transition: opacity 0.2s ease, transform 0.2s ease;
}

.popup-enter-from,
.popup-leave-to {
  opacity: 0;
  transform: translateY(100%);
}
</style>
