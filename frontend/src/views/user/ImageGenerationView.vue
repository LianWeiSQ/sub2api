<template>
  <AppLayout>
    <div class="mx-auto flex min-h-[calc(100vh-9rem)] w-full max-w-6xl flex-col px-4 py-6 sm:px-6 lg:px-8">
      <div class="mb-5 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">{{ t('imageGeneration.title') }}</h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ t('imageGeneration.description') }}</p>
        </div>
        <button
          class="btn btn-secondary"
          :disabled="loadingKey"
          :title="t('imageGeneration.refreshKey')"
          @click="loadDefaultKey"
        >
          <Icon name="refresh" size="sm" :class="loadingKey ? 'animate-spin' : ''" />
          <span>{{ activeKeyName }}</span>
        </button>
      </div>

      <div class="flex flex-1 flex-col overflow-hidden rounded-2xl border border-gray-200 bg-white shadow-sm dark:border-dark-700 dark:bg-dark-900">
        <div ref="resultArea" class="min-h-0 flex-1 overflow-y-auto px-4 py-5 sm:px-6">
          <div v-if="results.length === 0 && !generating" class="flex min-h-[360px] flex-col items-center justify-center text-center">
            <div class="flex h-14 w-14 items-center justify-center rounded-2xl bg-primary-50 text-primary-600 dark:bg-primary-500/10 dark:text-primary-300">
              <Icon name="sparkles" size="lg" />
            </div>
            <h2 class="mt-4 text-lg font-semibold text-gray-900 dark:text-white">{{ t('imageGeneration.emptyTitle') }}</h2>
            <p class="mt-2 max-w-md text-sm leading-6 text-gray-500 dark:text-dark-400">{{ t('imageGeneration.emptyDescription') }}</p>
          </div>

          <div class="space-y-5">
            <div v-for="item in results" :key="item.id" class="flex flex-col gap-3">
              <div class="flex justify-end">
                <div class="max-w-2xl rounded-2xl bg-primary-600 px-4 py-3 text-sm leading-6 text-white shadow-sm">
                  {{ item.prompt }}
                </div>
              </div>

              <div class="flex justify-start">
                <div class="w-full max-w-xl rounded-2xl border border-gray-200 bg-gray-50 p-3 dark:border-dark-700 dark:bg-dark-800/70">
                  <div class="relative aspect-square overflow-hidden rounded-xl bg-white dark:bg-dark-900">
                    <img
                      :src="item.url"
                      :alt="item.prompt"
                      class="h-full w-full object-contain"
                    />
                  </div>
                  <div class="mt-3 flex flex-wrap items-center justify-between gap-2">
                    <span class="text-xs text-gray-500 dark:text-dark-400">
                      {{ t('imageGeneration.completedIn', { seconds: formatSeconds(item.durationMs) }) }}
                    </span>
                    <a
                      :href="item.url"
                      :download="item.fileName"
                      class="btn btn-secondary btn-sm"
                    >
                      <Icon name="download" size="xs" />
                      <span>{{ t('imageGeneration.download') }}</span>
                    </a>
                  </div>
                </div>
              </div>
            </div>

            <div v-if="generating" class="flex justify-start">
              <div class="w-full max-w-xl rounded-2xl border border-gray-200 bg-gray-50 p-3 dark:border-dark-700 dark:bg-dark-800/70">
                <div class="flex aspect-square items-center justify-center rounded-xl bg-white dark:bg-dark-900">
                  <div class="flex flex-col items-center gap-3 text-center">
                    <Icon name="sync" size="lg" class="animate-spin text-primary-500" />
                    <div>
                      <div class="text-sm font-medium text-gray-900 dark:text-white">{{ t('imageGeneration.generating') }}</div>
                      <div class="mt-1 text-xs tabular-nums text-gray-500 dark:text-dark-400">
                        {{ t('imageGeneration.elapsed', { seconds: elapsedSeconds }) }}
                      </div>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>

        <form class="border-t border-gray-200 bg-gray-50 p-3 dark:border-dark-700 dark:bg-dark-950/60" @submit.prevent="handleGenerate">
          <div v-if="errorMessage" class="mb-3 rounded-xl border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700 dark:border-red-900/60 dark:bg-red-950/40 dark:text-red-200">
            {{ errorMessage }}
          </div>

          <div v-if="!defaultKey && !loadingKey" class="mb-3 flex flex-col gap-3 rounded-xl border border-amber-200 bg-amber-50 px-3 py-3 text-sm text-amber-800 dark:border-amber-900/60 dark:bg-amber-950/40 dark:text-amber-100 sm:flex-row sm:items-center sm:justify-between">
            <span>{{ t('imageGeneration.noKey') }}</span>
            <RouterLink to="/keys" class="btn btn-secondary btn-sm">
              <Icon name="key" size="xs" />
              <span>{{ t('imageGeneration.createKey') }}</span>
            </RouterLink>
          </div>

          <div class="flex items-end gap-2 rounded-2xl border border-gray-200 bg-white p-2 shadow-sm dark:border-dark-700 dark:bg-dark-900">
            <textarea
              v-model="prompt"
              rows="1"
              class="min-h-[44px] flex-1 resize-none border-0 bg-transparent px-3 py-2 text-sm leading-6 text-gray-900 outline-none placeholder:text-gray-400 focus:ring-0 dark:text-white dark:placeholder:text-dark-500"
              :placeholder="t('imageGeneration.promptPlaceholder')"
              :disabled="generating"
              @keydown.enter.exact.prevent="handleGenerate"
            />
            <button
              type="submit"
              class="btn btn-primary h-11 w-11 flex-shrink-0 p-0"
              :disabled="!canGenerate"
              :title="t('imageGeneration.generate')"
            >
              <Icon v-if="generating" name="sync" size="sm" class="animate-spin" />
              <Icon v-else name="arrowUp" size="sm" />
            </button>
          </div>
        </form>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import axios from 'axios'
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import {
  generateImage,
  getDefaultImageApiKey,
  imageToDataUrl,
  type GeneratedImage,
} from '@/api/imageGeneration'
import type { ApiKey } from '@/types'
import { extractApiErrorMessage } from '@/utils/apiError'

interface ImageResult {
  id: string
  prompt: string
  url: string
  durationMs: number
  fileName: string
}

const { t } = useI18n()

const defaultKey = ref<ApiKey | null>(null)
const loadingKey = ref(false)
const generating = ref(false)
const prompt = ref('')
const errorMessage = ref('')
const elapsedSeconds = ref(0)
const results = ref<ImageResult[]>([])
const resultArea = ref<HTMLElement | null>(null)

let keyAbortController: AbortController | null = null
let generationAbortController: AbortController | null = null
let timer: number | null = null

const activeKeyName = computed(() => {
  if (loadingKey.value) return t('imageGeneration.loadingKey')
  return defaultKey.value?.name || t('imageGeneration.noActiveKey')
})

const canGenerate = computed(() => {
  return Boolean(defaultKey.value?.key && prompt.value.trim() && !generating.value)
})

function formatSeconds(ms: number): string {
  return Math.max(1, Math.round(ms / 1000)).toString()
}

function startTimer() {
  elapsedSeconds.value = 0
  stopTimer()
  timer = window.setInterval(() => {
    elapsedSeconds.value += 1
  }, 1000)
}

function stopTimer() {
  if (timer !== null) {
    window.clearInterval(timer)
    timer = null
  }
}

async function scrollToBottom() {
  await nextTick()
  const el = resultArea.value
  if (el) {
    el.scrollTop = el.scrollHeight
  }
}

async function loadDefaultKey() {
  keyAbortController?.abort()
  keyAbortController = new AbortController()
  loadingKey.value = true
  errorMessage.value = ''
  try {
    defaultKey.value = await getDefaultImageApiKey(keyAbortController.signal)
  } catch (err: unknown) {
    if (axios.isCancel(err)) return
    errorMessage.value = extractApiErrorMessage(err, t('imageGeneration.keyLoadFailed'))
  } finally {
    loadingKey.value = false
  }
}

function buildResult(promptText: string, image: GeneratedImage, durationMs: number): ImageResult | null {
  const url = imageToDataUrl(image)
  if (!url) return null
  const id = `${Date.now()}-${Math.random().toString(36).slice(2)}`
  return {
    id,
    prompt: promptText,
    url,
    durationMs,
    fileName: `image-${id}.png`,
  }
}

function imageErrorMessage(err: unknown, fallback: string): string {
  if (
    axios.isAxiosError(err)
    && (err.code === 'ECONNABORTED' || err.message.toLowerCase().includes('timeout'))
  ) {
    return t('imageGeneration.timeoutMessage')
  }

  const openAIMessage = (err as { response?: { data?: { error?: { message?: string } } } })?.response?.data?.error?.message
  return openAIMessage || extractApiErrorMessage(err, fallback)
}

async function handleGenerate() {
  const key = defaultKey.value?.key
  const promptText = prompt.value.trim()
  if (!key || !promptText || generating.value) return

  generationAbortController?.abort()
  generationAbortController = new AbortController()
  generating.value = true
  errorMessage.value = ''
  startTimer()
  await scrollToBottom()

  try {
    const result = await generateImage(key, promptText, generationAbortController.signal)
    const generated = (result.response.data || [])
      .map((image) => buildResult(promptText, image, result.durationMs))
      .filter((item): item is ImageResult => item !== null)

    if (generated.length === 0) {
      errorMessage.value = t('imageGeneration.emptyResponse')
      return
    }

    results.value.push(...generated)
    prompt.value = ''
    await scrollToBottom()
  } catch (err: unknown) {
    if (axios.isCancel(err)) return
    errorMessage.value = imageErrorMessage(err, t('imageGeneration.generateFailed'))
  } finally {
    generating.value = false
    stopTimer()
  }
}

onMounted(loadDefaultKey)

onBeforeUnmount(() => {
  keyAbortController?.abort()
  generationAbortController?.abort()
  stopTimer()
})
</script>
