<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'

import FlowNode from '@/components/workflow/FlowNode.vue'
import FlowSequence from '@/components/workflow/FlowSequence.vue'
import type { WorkflowAsyncFlow } from '@/types/workflow'
import { stepTypesFor } from '@/utils/workflow'

const props = defineProps<{ flows: WorkflowAsyncFlow[] }>()

/** Link origin: the midpoint of the fork node's right edge, matching half the node card height. */
const ORIGIN_Y = 32
const root = ref<HTMLDivElement>()
const paths = ref<string[]>([])
const markerID = `fork-arrow-${Math.random().toString(36).slice(2, 8)}`

/** Connect the first flow horizontally; further flows branch from a shared vertical line. */
function measure() {
  const el = root.value
  if (!el) return
  const starts = el.querySelectorAll<HTMLElement>('[data-fork-start] > .node-row > .node-col > .flow-node')
  paths.value = [...starts].map((card) => {
    let x = -4
    let y = card.offsetHeight / 2
    for (let node: HTMLElement | null = card; node && node !== el; node = node.offsetParent as HTMLElement | null) {
      x += node.offsetLeft
      y += node.offsetTop
    }
    return y === ORIGIN_Y ? `M 0 ${ORIGIN_Y} H ${x}` : `M 0 ${ORIGIN_Y} H 36 V ${y} H ${x}`
  })
}

let observer: ResizeObserver | undefined
onMounted(() => {
  measure()
  observer = new ResizeObserver(() => measure())
  observer.observe(root.value!)
})
onBeforeUnmount(() => observer?.disconnect())
watch(
  () => props.flows.map((f) => f.id).join(),
  () => void nextTick(measure),
)
</script>

<template>
  <div ref="root" class="fork">
    <svg class="fork-links" aria-hidden="true">
      <defs>
        <marker :id="markerID" viewBox="0 0 10 10" refX="5" refY="5" markerWidth="7" markerHeight="7" orient="auto-start-reverse">
          <path d="M 1 1 L 9 5 L 1 9 z" class="arrow" />
        </marker>
      </defs>
      <path v-for="(d, i) in paths" :key="i" :d="d" class="link" :marker-end="`url(#${markerID})`" />
    </svg>
    <div class="fork-cols">
      <div v-for="flow in flows" :key="flow.id" class="fork-col">
        <div data-fork-start>
          <FlowNode :async-flow-id="flow.id" />
        </div>
        <FlowSequence :steps="flow.steps" :address="{ flow: flow.id }" :types="stepTypesFor('async')" branch />
      </div>
    </div>
  </div>
</template>

<style scoped>
.fork {
  position: relative;
  padding: 0 0 0 72px;
}
.fork-links {
  position: absolute;
  left: 0;
  top: 0;
  width: 1px;
  height: 1px;
  overflow: visible;
  pointer-events: none;
}
.link {
  fill: none;
  stroke: var(--line-border-strong);
  stroke-width: 2;
}
.arrow {
  fill: var(--line-border-strong);
}
.fork-cols {
  display: flex;
  flex-direction: column;
  gap: 48px;
}
.fork-col {
  display: flex;
  flex-direction: column;
  align-items: center;
}
.fork-col > :deep(.flow-seq) {
  flex: none;
}
</style>
