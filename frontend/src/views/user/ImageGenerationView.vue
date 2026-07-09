<template>
  <AppLayout>
    <div class="mx-auto flex min-h-[calc(100vh-8rem)] w-full max-w-[1500px] flex-col px-3 py-4 sm:px-5 lg:px-8">
      <div class="mb-4 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">{{ t('imageGeneration.title') }}</h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ t('imageGeneration.description') }}</p>
        </div>
        <div class="flex flex-wrap items-center gap-2">
          <button
            type="button"
            class="btn btn-secondary"
            @click="triggerUpload"
          >
            <Icon name="upload" size="sm" />
            <span>{{ t('imageGeneration.uploadImage') }}</span>
          </button>
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
      </div>

      <div class="flex flex-1 flex-col overflow-hidden rounded-2xl border border-gray-200 bg-white shadow-sm dark:border-dark-700 dark:bg-dark-900">
        <div ref="resultArea" class="min-h-0 flex-1 overflow-y-auto bg-slate-50/80 px-3 py-4 dark:bg-dark-950/40 sm:px-5">
          <div class="grid gap-4 lg:grid-cols-[230px_minmax(0,1fr)] xl:grid-cols-[220px_minmax(0,1fr)_310px] 2xl:grid-cols-[250px_minmax(0,1fr)_340px]">
            <aside class="lg:sticky lg:top-0 lg:self-start">
              <section class="rounded-2xl border border-gray-200 bg-white p-3 shadow-sm dark:border-dark-700 dark:bg-dark-900">
                <div class="flex items-center justify-between gap-3">
                  <h2 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('imageGeneration.templates.panelTitle') }}</h2>
                  <button
                    v-if="selectedTemplate"
                    type="button"
                    class="rounded-full p-1.5 text-gray-400 transition hover:bg-gray-100 hover:text-gray-700 dark:text-dark-400 dark:hover:bg-dark-800 dark:hover:text-dark-200"
                    :title="t('imageGeneration.templates.clear')"
                    @click="clearTemplateSelection"
                  >
                    <Icon name="x" size="xs" />
                  </button>
                </div>
                <p class="mt-1 text-xs leading-5 text-gray-500 dark:text-dark-400">{{ t('imageGeneration.templates.description') }}</p>

                <div class="mt-3 space-y-2">
                  <button
                    v-for="template in promptTemplates"
                    :key="template.id"
                    type="button"
                    class="group flex w-full items-start gap-3 rounded-xl border p-3 text-left transition hover:border-primary-200 hover:bg-primary-50/40 dark:hover:border-primary-700 dark:hover:bg-primary-500/5"
                    :class="selectedTemplateId === template.id
                      ? 'border-primary-400 bg-primary-50 shadow-sm dark:border-primary-500 dark:bg-primary-500/10'
                      : 'border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-900'"
                    @click="selectTemplate(template.id)"
                  >
                    <div
                      class="flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-lg"
                      :class="template.iconClass"
                    >
                      <Icon :name="template.icon" size="sm" />
                    </div>
                    <div class="min-w-0 flex-1">
                      <div class="flex min-w-0 items-center justify-between gap-2">
                        <div class="truncate text-sm font-semibold text-gray-900 dark:text-white">{{ template.name }}</div>
                        <span
                          class="flex-shrink-0 rounded-full px-2 py-0.5 text-[11px] font-medium"
                          :class="template.badgeClass"
                        >
                          {{ template.badge }}
                        </span>
                      </div>
                      <div class="mt-1 line-clamp-2 text-xs leading-5 text-gray-500 dark:text-dark-400">{{ template.description }}</div>
                    </div>
                  </button>
                </div>
              </section>
            </aside>

            <main class="min-w-0">
              <section class="min-h-[420px] rounded-2xl border border-gray-200 bg-white p-3 shadow-sm dark:border-dark-700 dark:bg-dark-900 lg:min-h-[560px]">
                <div class="mb-3 flex flex-wrap items-center justify-between gap-2 border-b border-gray-100 pb-3 dark:border-dark-800">
                  <div class="flex items-center gap-2">
                    <div class="flex h-8 w-8 items-center justify-center rounded-lg bg-gray-100 text-gray-600 dark:bg-dark-800 dark:text-dark-300">
                      <Icon name="sparkles" size="xs" />
                    </div>
                    <div>
                      <div class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('imageGeneration.canvasTitle') }}</div>
                      <div class="text-xs text-gray-500 dark:text-dark-400">{{ selectedTemplate ? selectedTemplate.name : t('imageGeneration.freeCreate') }}</div>
                    </div>
                  </div>
                  <div class="flex items-center gap-2 text-xs text-gray-500 dark:text-dark-400">
                    <span v-if="activeSource" class="rounded-full bg-emerald-50 px-2 py-1 font-medium text-emerald-700 dark:bg-emerald-500/10 dark:text-emerald-200">
                      {{ t('imageGeneration.editingMode') }}
                    </span>
                    <span class="rounded-full bg-gray-100 px-2 py-1 dark:bg-dark-800">
                      {{ activeKeyName }}
                    </span>
                  </div>
                </div>

                <div v-if="results.length === 0 && !generating" class="flex min-h-[340px] flex-col items-center justify-center rounded-2xl border border-dashed border-gray-200 bg-slate-50/70 px-6 py-12 text-center dark:border-dark-700 dark:bg-dark-950/40 lg:min-h-[470px]">
                  <div class="flex h-14 w-14 items-center justify-center rounded-2xl bg-primary-50 text-primary-600 dark:bg-primary-500/10 dark:text-primary-300">
                    <Icon name="sparkles" size="lg" />
                  </div>
                  <h2 class="mt-4 text-lg font-semibold text-gray-900 dark:text-white">{{ t('imageGeneration.emptyTitle') }}</h2>
                  <p class="mt-2 max-w-md text-sm leading-6 text-gray-500 dark:text-dark-400">{{ t('imageGeneration.emptyDescription') }}</p>
                  <div class="mt-5 flex flex-wrap justify-center gap-2">
                    <button type="button" class="btn btn-primary btn-sm" @click="focusPrompt">
                      <Icon name="sparkles" size="xs" />
                      <span>{{ t('imageGeneration.startFromPrompt') }}</span>
                    </button>
                    <button type="button" class="btn btn-secondary btn-sm" @click="triggerUpload">
                      <Icon name="upload" size="xs" />
                      <span>{{ t('imageGeneration.uploadToEdit') }}</span>
                    </button>
                  </div>
                </div>

                <div class="mx-auto max-w-4xl space-y-5">
                  <div v-for="item in results" :key="item.id" class="flex flex-col gap-3">
                    <div class="flex justify-end">
                      <div class="max-w-2xl rounded-2xl bg-primary-600 px-4 py-3 text-sm leading-6 text-white shadow-sm">
                        {{ item.prompt }}
                      </div>
                    </div>

                    <div class="flex justify-start">
                      <div class="w-full max-w-2xl rounded-2xl border border-gray-200 bg-white p-3 shadow-sm dark:border-dark-700 dark:bg-dark-900">
                        <div class="relative aspect-square overflow-hidden rounded-xl bg-gray-50 dark:bg-dark-950">
                          <img
                            :src="item.url"
                            :alt="item.prompt"
                            class="h-full w-full object-contain"
                          />
                        </div>
                        <div class="mt-3 flex flex-wrap items-center justify-between gap-2">
                          <span class="text-xs text-gray-500 dark:text-dark-400">
                            {{ resultMeta(item) }}
                          </span>
                          <div class="flex flex-wrap items-center gap-2">
                            <button
                              type="button"
                              class="btn btn-secondary btn-sm"
                              :disabled="!item.sourceBlob"
                              @click="selectResultForEdit(item)"
                            >
                              <Icon name="edit" size="xs" />
                              <span>{{ t('imageGeneration.continueEdit') }}</span>
                            </button>
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
                  </div>

                  <div v-if="generating" class="flex justify-start">
                    <div class="w-full max-w-2xl rounded-2xl border border-gray-200 bg-white p-3 shadow-sm dark:border-dark-700 dark:bg-dark-900">
                      <div class="flex aspect-square items-center justify-center rounded-xl bg-gray-50 dark:bg-dark-950">
                        <div class="flex flex-col items-center gap-3 text-center">
                          <Icon name="sync" size="lg" class="animate-spin text-primary-500" />
                          <div>
                            <div class="text-sm font-medium text-gray-900 dark:text-white">
                              {{ activeSource ? t('imageGeneration.editing') : t('imageGeneration.generating') }}
                            </div>
                            <div class="mt-1 text-xs tabular-nums text-gray-500 dark:text-dark-400">
                              {{ t('imageGeneration.elapsed', { seconds: elapsedSeconds }) }}
                            </div>
                          </div>
                        </div>
                      </div>
                    </div>
                  </div>
                </div>
              </section>
            </main>

            <aside class="lg:col-span-2 xl:sticky xl:top-0 xl:col-span-1 xl:self-start">
              <section class="rounded-2xl border border-gray-200 bg-white p-4 shadow-sm dark:border-dark-700 dark:bg-dark-900">
                <div class="flex items-center justify-between gap-3">
                  <h2 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('imageGeneration.templates.settingsTitle') }}</h2>
                  <button
                    v-if="selectedTemplate"
                    type="button"
                    class="btn btn-primary btn-sm"
                    @click="applyTemplatePrompt()"
                  >
                    <Icon name="sparkles" size="xs" />
                    <span>{{ t('imageGeneration.templates.apply') }}</span>
                  </button>
                </div>

                <template v-if="selectedTemplate">
                  <div class="mt-4 flex items-start gap-3 rounded-xl border border-primary-100 bg-primary-50/70 p-3 dark:border-primary-900/50 dark:bg-primary-500/10">
                    <div class="flex h-11 w-11 flex-shrink-0 items-center justify-center rounded-lg" :class="selectedTemplate.iconClass">
                      <Icon :name="selectedTemplate.icon" size="sm" />
                    </div>
                    <div class="min-w-0 flex-1">
                      <div class="flex flex-wrap items-center gap-2">
                        <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ selectedTemplate.name }}</h3>
                        <span class="rounded-full px-2 py-0.5 text-[11px] font-medium" :class="selectedTemplate.badgeClass">
                          {{ selectedTemplate.badge }}
                        </span>
                      </div>
                      <p class="mt-1 text-xs leading-5 text-gray-600 dark:text-dark-300">{{ selectedTemplate.workflow }}</p>
                    </div>
                  </div>

                  <div
                    v-if="selectedTemplate.fields.length > 0"
                    class="mt-4 space-y-3"
                  >
                    <label
                      v-for="field in selectedTemplate.fields"
                      :key="field.key"
                      class="flex min-w-0 flex-col gap-1.5"
                    >
                      <span class="text-xs font-medium text-gray-600 dark:text-dark-300">{{ field.label }}</span>
                      <select
                        v-if="field.type === 'select'"
                        v-model="templateForm[field.key]"
                        class="h-10 rounded-lg border border-gray-200 bg-gray-50 px-3 text-sm text-gray-900 outline-none focus:border-primary-400 focus:bg-white focus:ring-2 focus:ring-primary-100 dark:border-dark-700 dark:bg-dark-950 dark:text-white dark:focus:border-primary-500 dark:focus:bg-dark-900 dark:focus:ring-primary-900/40"
                      >
                        <option
                          v-for="option in field.options"
                          :key="option"
                          :value="option"
                        >
                          {{ option }}
                        </option>
                      </select>
                      <input
                        v-else
                        v-model="templateForm[field.key]"
                        type="text"
                        class="h-10 rounded-lg border border-gray-200 bg-gray-50 px-3 text-sm text-gray-900 outline-none placeholder:text-gray-400 focus:border-primary-400 focus:bg-white focus:ring-2 focus:ring-primary-100 dark:border-dark-700 dark:bg-dark-950 dark:text-white dark:placeholder:text-dark-500 dark:focus:border-primary-500 dark:focus:bg-dark-900 dark:focus:ring-primary-900/40"
                        :placeholder="field.placeholder"
                      />
                    </label>
                  </div>

                  <div v-if="selectedTemplate.quickActions.length > 0" class="mt-4">
                    <div class="mb-2 text-xs font-medium text-gray-600 dark:text-dark-300">{{ t('imageGeneration.templates.quickActions') }}</div>
                    <div class="flex flex-wrap gap-2">
                      <button
                        v-for="action in selectedTemplate.quickActions"
                        :key="action.label"
                        type="button"
                        class="rounded-full border border-gray-200 bg-white px-3 py-1.5 text-xs font-medium text-gray-600 transition hover:border-primary-300 hover:text-primary-700 dark:border-dark-700 dark:bg-dark-900 dark:text-dark-300 dark:hover:border-primary-600 dark:hover:text-primary-200"
                        @click="appendPromptLine(action.prompt)"
                      >
                        {{ action.label }}
                      </button>
                    </div>
                  </div>

                  <div class="mt-4 rounded-xl border border-gray-200 bg-slate-50 p-3 dark:border-dark-700 dark:bg-dark-950/60">
                    <div class="mb-2 flex items-center justify-between gap-2">
                      <span class="text-xs font-medium text-gray-600 dark:text-dark-300">{{ t('imageGeneration.templates.promptDraft') }}</span>
                      <span class="text-[11px] text-gray-400 dark:text-dark-500">
                        {{ t('imageGeneration.templates.characters', { count: templatePromptPreview.length }) }}
                      </span>
                    </div>
                    <pre class="max-h-52 whitespace-pre-wrap break-words text-xs leading-5 text-gray-600 dark:text-dark-300">{{ templatePromptPreview }}</pre>
                  </div>
                </template>

                <div v-else class="mt-4 rounded-xl border border-dashed border-gray-200 bg-slate-50 p-4 text-sm text-gray-500 dark:border-dark-700 dark:bg-dark-950/50 dark:text-dark-400">
                  <div class="font-medium text-gray-700 dark:text-dark-200">{{ t('imageGeneration.templates.emptyTitle') }}</div>
                  <p class="mt-1 text-xs leading-5">{{ t('imageGeneration.templates.emptyDescription') }}</p>
                </div>
              </section>
            </aside>
          </div>
        </div>

        <form class="border-t border-gray-200 bg-white p-3 dark:border-dark-700 dark:bg-dark-900" @submit.prevent="handleGenerate">
          <input
            ref="fileInput"
            type="file"
            accept="image/png,image/jpeg,image/webp"
            class="hidden"
            @change="handleUpload"
          />

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

          <div v-if="activeSource" class="mb-3 rounded-2xl border border-primary-200 bg-primary-50/60 p-3 shadow-sm dark:border-primary-900/60 dark:bg-primary-500/10">
            <div class="flex flex-col gap-3 md:flex-row md:items-start">
              <div class="flex min-w-0 items-start gap-3 md:w-72 md:flex-shrink-0">
                <div class="flex h-20 w-20 flex-shrink-0 items-center justify-center overflow-hidden rounded-xl border border-gray-200 bg-gray-100 text-gray-400 dark:border-dark-700 dark:bg-dark-800 dark:text-dark-400">
                  <img
                    v-if="!sourcePreviewFailed"
                    :src="activeSource.previewUrl"
                    alt=""
                    class="h-full w-full object-cover"
                    @load="handleActiveSourcePreviewLoad"
                    @error="handleActiveSourcePreviewError"
                  />
                  <Icon v-else name="upload" size="md" />
                </div>
                <div class="min-w-0 flex-1">
                  <div class="text-xs font-medium uppercase tracking-wide text-primary-600 dark:text-primary-300">
                    {{ t('imageGeneration.editSource') }}
                  </div>
                  <div class="mt-1 max-w-full truncate text-sm font-medium text-gray-900 dark:text-white" :title="activeSource.fileName">
                    {{ activeSource.label }}
                  </div>
                  <div class="mt-1 text-xs text-gray-500 dark:text-dark-400">
                    {{ maskDirty ? t('imageGeneration.maskReady') : t('imageGeneration.editSourceHint') }}
                  </div>
                </div>
              </div>

              <div class="flex min-w-0 flex-1 flex-col gap-3">
                <div class="flex flex-wrap items-center gap-2">
                  <button
                    type="button"
                    class="btn btn-secondary btn-sm"
                    :class="maskMode ? 'ring-2 ring-primary-400 dark:ring-primary-500' : ''"
                    @click="toggleMaskMode"
                  >
                    <Icon name="edit" size="xs" />
                    <span>{{ maskMode ? t('imageGeneration.closeMaskEditor') : t('imageGeneration.paintMask') }}</span>
                  </button>
                  <button type="button" class="btn btn-secondary btn-sm" @click="clearActiveSource">
                    <Icon name="x" size="xs" />
                    <span>{{ t('imageGeneration.newImage') }}</span>
                  </button>
                  <button v-if="maskDirty" type="button" class="btn btn-secondary btn-sm" @click="clearMask">
                    <Icon name="trash" size="xs" />
                    <span>{{ t('imageGeneration.clearMask') }}</span>
                  </button>
                </div>

                <div v-show="maskMode" class="rounded-xl border border-gray-200 bg-gray-50 p-3 dark:border-dark-700 dark:bg-dark-800/70">
                  <div class="mb-3 flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
                    <span class="text-xs text-gray-500 dark:text-dark-400">{{ t('imageGeneration.maskHint') }}</span>
                    <label class="flex items-center gap-2 text-xs text-gray-500 dark:text-dark-400">
                      <span>{{ t('imageGeneration.brushSize') }}</span>
                      <input v-model.number="brushSize" type="range" min="12" max="96" step="2" class="w-28 accent-primary-500" />
                    </label>
                  </div>
                  <div class="overflow-hidden rounded-lg bg-dark-950/5 dark:bg-dark-950">
                    <canvas
                      ref="previewCanvas"
                      class="block max-h-[420px] w-full cursor-crosshair touch-none"
                      @pointerdown="startPaint"
                      @pointermove="paint"
                      @pointerup="stopPaint"
                      @pointerleave="stopPaint"
                      @pointercancel="stopPaint"
                    ></canvas>
                    <canvas ref="maskCanvas" class="hidden"></canvas>
                  </div>
                </div>
              </div>
            </div>
          </div>

          <div class="mb-2 flex flex-wrap items-center justify-between gap-2 px-1">
            <div class="flex min-w-0 flex-wrap items-center gap-2">
              <span
                v-if="selectedTemplate"
                class="inline-flex max-w-full items-center gap-1 rounded-full bg-primary-50 px-2.5 py-1 text-xs font-medium text-primary-700 ring-1 ring-primary-100 dark:bg-primary-500/10 dark:text-primary-200 dark:ring-primary-900/60"
              >
                <Icon name="sparkles" size="xs" />
                <span class="truncate">{{ selectedTemplate.name }}</span>
              </span>
              <span
                v-if="activeSource"
                class="inline-flex items-center gap-1 rounded-full bg-emerald-50 px-2.5 py-1 text-xs font-medium text-emerald-700 ring-1 ring-emerald-100 dark:bg-emerald-500/10 dark:text-emerald-200 dark:ring-emerald-900/60"
              >
                <Icon name="edit" size="xs" />
                <span>{{ t('imageGeneration.editingMode') }}</span>
              </span>
            </div>
            <button
              type="button"
              class="inline-flex min-w-0 items-center gap-1.5 rounded-full px-2.5 py-1 text-xs font-medium text-gray-500 transition hover:bg-gray-100 hover:text-gray-700 dark:text-dark-400 dark:hover:bg-dark-800 dark:hover:text-dark-200"
              :disabled="loadingKey"
              :title="t('imageGeneration.refreshKey')"
              @click="loadDefaultKey"
            >
              <Icon name="refresh" size="xs" :class="loadingKey ? 'animate-spin' : ''" />
              <span class="max-w-40 truncate">{{ activeKeyName }}</span>
            </button>
          </div>

          <div class="flex items-end gap-2 rounded-[1.5rem] border border-gray-200 bg-gray-50 p-2 shadow-sm transition focus-within:border-primary-300 focus-within:bg-white focus-within:ring-2 focus-within:ring-primary-100 dark:border-dark-700 dark:bg-dark-950 dark:focus-within:border-primary-600 dark:focus-within:bg-dark-900 dark:focus-within:ring-primary-900/30">
            <button
              type="button"
              class="btn btn-secondary h-11 w-11 flex-shrink-0 p-0"
              :title="t('imageGeneration.uploadImage')"
              @click="triggerUpload"
            >
              <Icon name="upload" size="sm" />
            </button>
            <textarea
              ref="promptInput"
              v-model="prompt"
              rows="1"
              class="max-h-40 min-h-[44px] flex-1 resize-none border-0 bg-transparent px-3 py-2 text-sm leading-6 text-gray-900 outline-none placeholder:text-gray-400 focus:ring-0 dark:text-white dark:placeholder:text-dark-500"
              :placeholder="activeSource ? t('imageGeneration.editPromptPlaceholder') : t('imageGeneration.promptPlaceholder')"
              :disabled="generating"
              @input="resizePromptInput"
              @keydown.enter.exact.prevent="handleGenerate"
            />
            <button
              type="submit"
              class="btn btn-primary h-11 w-11 flex-shrink-0 p-0"
              :disabled="!canGenerate"
              :title="activeSource ? t('imageGeneration.edit') : t('imageGeneration.generate')"
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
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { RouterLink } from 'vue-router'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import {
  editImage,
  generateImage,
  getDefaultImageApiKey,
  imageToBlob,
  imageToDataUrl,
  type GeneratedImage,
} from '@/api/imageGeneration'
import type { ApiKey } from '@/types'
import { extractApiErrorMessage } from '@/utils/apiError'

type ImageMode = 'generate' | 'edit'
type SourceOrigin = 'upload' | 'result'

interface ImageResult {
  id: string
  prompt: string
  url: string
  durationMs: number
  fileName: string
  mode: ImageMode
  sourceBlob: Blob | null
}

interface ActiveSource {
  blob: Blob
  fileName: string
  previewUrl: string
  label: string
  origin: SourceOrigin
  revokeUrl: boolean
}

interface Point {
  x: number
  y: number
}

type TemplateId =
  | 'commercialPoster'
  | 'xiaohongshu'
  | 'wechat'
  | 'appMockup'
  | 'saasDashboard'
  | 'dataViz'
  | 'researchDiagram'
  | 'productPhoto'
  | 'brandKit'
  | 'sceneStaging'
  | 'inkChinese'
  | 'cyberpunkHud'

type TemplateIcon =
  | 'sparkles'
  | 'fire'
  | 'book'
  | 'user'
  | 'chart'
  | 'chartBar'
  | 'cpu'
  | 'cube'
  | 'badge'
  | 'upload'
  | 'cloud'
  | 'bolt'

type TemplateFieldType = 'text' | 'select'

interface TemplateField {
  key: string
  label: string
  placeholder?: string
  type: TemplateFieldType
  options?: string[]
}

interface TemplateAction {
  label: string
  prompt: string
}

interface PromptTemplate {
  id: TemplateId
  name: string
  badge: string
  description: string
  workflow: string
  icon: TemplateIcon
  iconClass: string
  badgeClass: string
  fields: TemplateField[]
  defaults: Record<string, string>
  quickActions: TemplateAction[]
  buildPrompt: (values: Record<string, string>) => string
}

const { t } = useI18n()

const defaultKey = ref<ApiKey | null>(null)
const loadingKey = ref(false)
const generating = ref(false)
const prompt = ref('')
const errorMessage = ref('')
const elapsedSeconds = ref(0)
const results = ref<ImageResult[]>([])
const activeSource = ref<ActiveSource | null>(null)
const sourcePreviewFailed = ref(false)
const maskMode = ref(false)
const maskDirty = ref(false)
const brushSize = ref(44)
const selectedTemplateId = ref<TemplateId | null>(null)
const templateForm = reactive<Record<string, string>>({})

const resultArea = ref<HTMLElement | null>(null)
const fileInput = ref<HTMLInputElement | null>(null)
const promptInput = ref<HTMLTextAreaElement | null>(null)
const previewCanvas = ref<HTMLCanvasElement | null>(null)
const maskCanvas = ref<HTMLCanvasElement | null>(null)

let keyAbortController: AbortController | null = null
let generationAbortController: AbortController | null = null
let timer: number | null = null
let painting = false
let lastPoint: Point | null = null

const simpleFields: TemplateField[] = [
  {
    key: 'topic',
    label: '主题',
    placeholder: '例如：AI 自动化工作流',
    type: 'text',
  },
  {
    key: 'mustText',
    label: '必须出现的文字',
    placeholder: '例如：真正能变现的是工作流',
    type: 'text',
  },
]

const xiaohongshuFields: TemplateField[] = [
  {
    key: 'format',
    label: '内容类型',
    type: 'select',
    options: ['爆款封面', '干货知识卡', '种草产品图', '对比避坑图', '系列图文卡片'],
  },
  {
    key: 'topic',
    label: '主题',
    placeholder: '例如：普通人怎么用 AI 做副业',
    type: 'text',
  },
  {
    key: 'audience',
    label: '目标人群',
    placeholder: '例如：职场人 / 创业者 / 宝妈',
    type: 'text',
  },
  {
    key: 'title',
    label: '主标题',
    placeholder: '例如：别再只会问 ChatGPT 了',
    type: 'text',
  },
  {
    key: 'tone',
    label: '标题风格',
    type: 'select',
    options: ['干货型', '冲突型', '温柔型', '专业型', '争议型'],
  },
  {
    key: 'visualStyle',
    label: '视觉风格',
    type: 'select',
    options: ['白底蓝黑科技风', '红黑强冲击', '奶油手账风', '杂志封面感', '极简知识卡'],
  },
  {
    key: 'mustText',
    label: '补充文字',
    placeholder: '例如：3 个可复制工作流',
    type: 'text',
  },
  {
    key: 'notes',
    label: '补充要求',
    placeholder: '例如：不要真人，突出工具感',
    type: 'text',
  },
]

const wechatFields: TemplateField[] = [
  {
    key: 'format',
    label: '图片用途',
    type: 'select',
    options: ['公众号首图', '文章内配图', '信息图卡片', '技术架构图', '文章金句图', '系列文章视觉模板'],
  },
  {
    key: 'title',
    label: '文章标题',
    placeholder: '例如：AI Agent 的下一站，不是聊天，而是交付',
    type: 'text',
  },
  {
    key: 'articleType',
    label: '文章类型',
    type: 'select',
    options: ['技术观点', '产品发布', '行业分析', '教程指南', '投研解读'],
  },
  {
    key: 'coreIdea',
    label: '核心观点',
    placeholder: '例如：从聊天界面走向可验证交付',
    type: 'text',
  },
  {
    key: 'keywords',
    label: '关键词',
    placeholder: '例如：Agent / 工具调用 / 记忆 / 验证',
    type: 'text',
  },
  {
    key: 'accountStyle',
    label: '账号风格',
    type: 'select',
    options: ['硅基技术栈：白底蓝黑专业科技', '投研报告风', '极简白底', '深色科技媒体', '杂志封面感'],
  },
  {
    key: 'mustText',
    label: '必须出现的文字',
    placeholder: '默认使用文章标题',
    type: 'text',
  },
]

function textValue(values: Record<string, string>, key: string, fallback = '') {
  return (values[key] || fallback).trim()
}

function compactPrompt(lines: Array<string | false | null | undefined>) {
  return lines.filter((line): line is string => Boolean(line && line.trim())).join('\n')
}

function buildSimplePrompt(
  artifact: string,
  topic: string,
  mustText: string,
  body: string[],
) {
  return compactPrompt([
    `任务：生成一张可直接交付的「${artifact}」。`,
    `主题：${topic || '请围绕用户主题创作'}。`,
    mustText ? `关键文字：画面必须准确显示 "${mustText}"。` : '',
    ...body,
    '交付标准：主体完整，层级清楚，中文清晰可读；不要乱码、不要随机 logo、不要无意义小字；整体像成熟设计初稿而不是模板拼贴。',
  ])
}

function buildXiaohongshuPrompt(values: Record<string, string>) {
  const format = textValue(values, 'format', '爆款封面')
  const topic = textValue(values, 'topic', '普通人怎么用 AI 做副业')
  const audience = textValue(values, 'audience', '职场人和创业者')
  const title = textValue(values, 'title', '别再只会问 ChatGPT 了')
  const tone = textValue(values, 'tone', '干货型')
  const visualStyle = textValue(values, 'visualStyle', '白底蓝黑科技风')
  const mustText = textValue(values, 'mustText', '真正能变现的是工作流')
  const notes = textValue(values, 'notes')

  return compactPrompt([
    `任务：为小红书生成「${format}」。`,
    `目标用户：${audience}。主题：${topic}。`,
    `画面目标：手机信息流里 1 秒能看懂，标题足够大，利益点清楚，但不要廉价营销感。`,
    `主标题：必须准确显示 "${title}"。标题风格：${tone}。`,
    mustText ? `辅助文字：准确显示 "${mustText}"，作为副标题或标签化卖点。` : '',
    `视觉方向：${visualStyle}。构图使用竖版封面感，主体和标题都放在安全区内。`,
    format === '爆款封面'
      ? '版式：主标题占上半部分，字号很大；底部放 2-3 个收益点标签；用一个强主视觉承接主题。'
      : '',
    format === '干货知识卡'
      ? '版式：顶部主标题，中间用 3 个清晰模块讲方法，底部保留栏目/作者区域；像可收藏的知识卡。'
      : '',
    format === '种草产品图'
      ? '版式：中心展示产品或服务概念，周围用卖点标签和信任感小组件包装；像高质量种草图，不要硬广堆字。'
      : '',
    format === '对比避坑图'
      ? '版式：左右对比结构，左侧显示错误做法，右侧显示正确做法；用清晰分隔线和红/绿提示强化判断。'
      : '',
    format === '系列图文卡片'
      ? '版式：生成系列首图，底部展示 "01/06" 系列标记，并暗示后续 5 张内页的模块主题。'
      : '',
    notes ? `补充要求：${notes}。` : '',
    '质检标准：缩略图状态下仍能读懂标题；中文不乱码；不要真实平台 logo、二维码、无关英文和过度拥挤的小字。',
  ])
}

function buildWechatPrompt(values: Record<string, string>) {
  const format = textValue(values, 'format', '公众号首图')
  const title = textValue(values, 'title', 'AI Agent 的下一站，不是聊天，而是交付')
  const articleType = textValue(values, 'articleType', '技术观点')
  const coreIdea = textValue(values, 'coreIdea', '从聊天界面走向可验证交付')
  const keywords = textValue(values, 'keywords', 'Agent / 工具调用 / 记忆 / 验证')
  const accountStyle = textValue(values, 'accountStyle', '硅基技术栈：白底蓝黑专业科技')
  const mustText = textValue(values, 'mustText', title)

  return compactPrompt([
    `任务：生成一张微信公众号「${format}」。`,
    `文章类型：${articleType}。账号风格：${accountStyle}。`,
    `标题文字：必须准确显示 "${mustText}"。`,
    `核心观点：${coreIdea}。`,
    `关键词：${keywords}。`,
    format === '公众号首图'
      ? '构图：适合公众号封面裁切，标题放在安全区，主视觉集中在中间横向区域，四周留出呼吸感和裁切安全边距。'
      : '',
    format === '文章内配图'
      ? '构图：像正文中的解释配图，使用清晰模块、箭头和短标签帮助读者理解，不要做成广告海报。'
      : '',
    format === '信息图卡片'
      ? '构图：一张图讲清一个概念，包含标题、三到五个要点、简洁图标和明确层级，适合插入公众号正文。'
      : '',
    format === '技术架构图'
      ? '构图：展示输入、模型/Agent、工具、记忆、验证、输出等模块，使用箭头连接，模块名称清晰可读，像专业技术文章架构图。'
      : '',
    format === '文章金句图'
      ? '构图：突出一句核心观点，留足留白，像可以转发朋友圈的专业观点卡，不要花哨。'
      : '',
    format === '系列文章视觉模板'
      ? '构图：生成统一栏目模板，包含栏目名、文章标题、期数位置、底部账号区域，便于后续系列复用。'
      : '',
    '质检标准：专业、可信、克制；标题在聊天列表缩略图里也能辨认；中文准确无乱码；不要真实公司 logo、假二维码和密集小字。',
  ])
}

const promptTemplates: PromptTemplate[] = [
  {
    id: 'commercialPoster',
    name: '商业海报',
    badge: '营销',
    description: '商品、活动、课程或服务宣传图，强调主视觉和卖点层级。',
    workflow: '适合电商首图、活动海报、产品发布图。输入主题和必须出现的文案后，会自动补齐商业海报层级。',
    icon: 'sparkles',
    iconClass: 'bg-amber-50 text-amber-600 dark:bg-amber-500/10 dark:text-amber-300',
    badgeClass: 'bg-amber-50 text-amber-700 dark:bg-amber-500/10 dark:text-amber-200',
    fields: simpleFields,
    defaults: {
      topic: 'AI 自动化训练营',
      mustText: '7 天搭建你的第一个 Agent 工作流',
    },
    quickActions: [
      { label: '更像高端发布会', prompt: '整体更像大厂新品发布会主视觉，克制、精致、可信。' },
      { label: '更强转化', prompt: '强化优惠/利益点区域，但不要廉价促销感。' },
    ],
    buildPrompt: values => buildSimplePrompt('商业海报', textValue(values, 'topic'), textValue(values, 'mustText'), [
      '版式：主标题最大，副标题解释价值，底部放 3 个卖点标签；中心有清晰产品/服务主视觉。',
      '风格：现代商业设计，干净背景，明确 CTA 区域，高级但有转化感。',
    ]),
  },
  {
    id: 'xiaohongshu',
    name: '小红书封面',
    badge: '重点',
    description: '爆款封面、知识卡、避坑对比和系列图文首图。',
    workflow: '面向手机信息流，强调 1 秒停留、中文大标题、标签化卖点和可收藏结构。',
    icon: 'fire',
    iconClass: 'bg-rose-50 text-rose-600 dark:bg-rose-500/10 dark:text-rose-300',
    badgeClass: 'bg-rose-50 text-rose-700 dark:bg-rose-500/10 dark:text-rose-200',
    fields: xiaohongshuFields,
    defaults: {
      format: '爆款封面',
      topic: '普通人怎么用 AI 做副业',
      audience: '职场人和创业者',
      title: '别再只会问 ChatGPT 了',
      tone: '干货型',
      visualStyle: '白底蓝黑科技风',
      mustText: '真正能变现的是工作流',
      notes: '不要真人，突出工具感和行动路径',
    },
    quickActions: [
      { label: '更强冲突感', prompt: '把视觉冲突加强，突出“普通人”和“高手”的差异，但保持专业。' },
      { label: '更像知识博主', prompt: '降低营销感，做成值得收藏的知识博主干货卡片。' },
      { label: '生成系列感', prompt: '加入系列栏目感，底部显示 01/06，并暗示后续内页。' },
    ],
    buildPrompt: buildXiaohongshuPrompt,
  },
  {
    id: 'wechat',
    name: '微信公众号图片生成',
    badge: '重点',
    description: '公众号首图、文内配图、信息图、技术架构图和金句图。',
    workflow: '面向公众号阅读和转发场景，强调专业可信、标题清晰、裁切安全和内容解释力。',
    icon: 'book',
    iconClass: 'bg-emerald-50 text-emerald-600 dark:bg-emerald-500/10 dark:text-emerald-300',
    badgeClass: 'bg-emerald-50 text-emerald-700 dark:bg-emerald-500/10 dark:text-emerald-200',
    fields: wechatFields,
    defaults: {
      format: '公众号首图',
      title: 'AI Agent 的下一站，不是聊天，而是交付',
      articleType: '技术观点',
      coreIdea: '从聊天界面走向可验证交付',
      keywords: 'Agent / 工具调用 / 记忆 / 验证',
      accountStyle: '硅基技术栈：白底蓝黑专业科技',
      mustText: '',
    },
    quickActions: [
      { label: '更像硅基技术栈', prompt: '整体更像专业技术公众号封面，白底蓝黑，信息密度适中。' },
      { label: '改成架构图', prompt: '把画面改成技术架构图，模块和箭头清晰可读。' },
      { label: '更适合转发', prompt: '强化标题识别和观点表达，让封面在聊天列表中也能看清。' },
    ],
    buildPrompt: buildWechatPrompt,
  },
  {
    id: 'appMockup',
    name: 'App UI Mockup',
    badge: '产品',
    description: '输入产品想法，生成真实 App 截图般的高保真界面。',
    workflow: '适合产品概念、融资材料、功能验证和 UI 灵感探索。',
    icon: 'user',
    iconClass: 'bg-sky-50 text-sky-600 dark:bg-sky-500/10 dark:text-sky-300',
    badgeClass: 'bg-sky-50 text-sky-700 dark:bg-sky-500/10 dark:text-sky-200',
    fields: simpleFields,
    defaults: {
      topic: 'AI 个人助理 App',
      mustText: 'Today Brief',
    },
    quickActions: [
      { label: '更像 iOS', prompt: '界面更像精致 iOS App，组件间距准确，图标克制。' },
      { label: '更像金融 App', prompt: '加入余额、趋势、交易记录等可信金融产品信息架构。' },
    ],
    buildPrompt: values => buildSimplePrompt('手机 App UI Mockup', textValue(values, 'topic'), textValue(values, 'mustText'), [
      '画面：高保真手机界面，包含顶部标题、核心数据卡片、功能入口、列表和底部导航。',
      '要求：像真实产品截图，组件对齐准确，字体清晰，数据合理，不要像营销落地页。',
    ]),
  },
  {
    id: 'saasDashboard',
    name: 'SaaS Dashboard',
    badge: '产品',
    description: '生成后台看板、运营分析、监控平台或管理控制台。',
    workflow: '适合产品原型、演示页、运营面板和数据产品概念图。',
    icon: 'chart',
    iconClass: 'bg-blue-50 text-blue-600 dark:bg-blue-500/10 dark:text-blue-300',
    badgeClass: 'bg-blue-50 text-blue-700 dark:bg-blue-500/10 dark:text-blue-200',
    fields: simpleFields,
    defaults: {
      topic: '模型网关运营监控平台',
      mustText: 'Gateway Overview',
    },
    quickActions: [
      { label: '更像大厂后台', prompt: '降低装饰感，提高信息密度和扫描效率，像成熟 SaaS 控制台。' },
      { label: '加入延迟指标', prompt: '加入 RPM、TPM、P50、P95、错误率、成本趋势等指标。' },
    ],
    buildPrompt: values => buildSimplePrompt('SaaS 数据看板', textValue(values, 'topic'), textValue(values, 'mustText'), [
      '布局：左侧导航、顶部过滤器、KPI 卡片、趋势图、表格和告警区域。',
      '风格：企业级后台，不要营销 hero；信息密集但有秩序，适合重复查看。',
    ]),
  },
  {
    id: 'dataViz',
    name: '数据可视化',
    badge: '图表',
    description: '行业地图、Sankey、热力图、趋势图和对比图。',
    workflow: '适合把复杂数据关系做成可读信息图；当前不接真实数据时会生成示意数据。',
    icon: 'chartBar',
    iconClass: 'bg-violet-50 text-violet-600 dark:bg-violet-500/10 dark:text-violet-300',
    badgeClass: 'bg-violet-50 text-violet-700 dark:bg-violet-500/10 dark:text-violet-200',
    fields: simpleFields,
    defaults: {
      topic: 'AI Agent 产业链机会地图',
      mustText: 'Agent Stack Opportunity Map',
    },
    quickActions: [
      { label: '做成行业地图', prompt: '使用象限图和分层产业链地图表达，标签清晰。' },
      { label: '做成 Sankey', prompt: '使用 Sankey 流向图表达资金、数据或请求流转。' },
    ],
    buildPrompt: values => buildSimplePrompt('数据可视化信息图', textValue(values, 'topic'), textValue(values, 'mustText'), [
      '结构：标题、图例、主图表、注释区；明确颜色含义和标签层级。',
      '要求：像专业媒体/研究报告图表，数字如为示意请避免伪造精确结论。',
    ]),
  },
  {
    id: 'researchDiagram',
    name: '科研/架构图',
    badge: '技术',
    description: '模型架构、Agent 流程、RAG 管线和论文方法图。',
    workflow: '适合公众号技术文章、论文草图、方案汇报和系统设计解释。',
    icon: 'cpu',
    iconClass: 'bg-indigo-50 text-indigo-600 dark:bg-indigo-500/10 dark:text-indigo-300',
    badgeClass: 'bg-indigo-50 text-indigo-700 dark:bg-indigo-500/10 dark:text-indigo-200',
    fields: simpleFields,
    defaults: {
      topic: '多 Agent 交付工作流',
      mustText: 'Plan → Tool Use → Verify → Deliver',
    },
    quickActions: [
      { label: '更像论文图', prompt: '采用白底、细线、模块框、编号流程，像 conference paper figure。' },
      { label: '更像系统架构', prompt: '加入 API Gateway、Queue、Worker、Memory、Evaluator 等模块。' },
    ],
    buildPrompt: values => buildSimplePrompt('技术架构图', textValue(values, 'topic'), textValue(values, 'mustText'), [
      '结构：输入、处理模块、工具/数据、验证、输出，使用箭头连接。',
      '要求：模块名称清晰可读，线条克制，像专业技术文档配图。',
    ]),
  },
  {
    id: 'productPhoto',
    name: '产品摄影',
    badge: '商品',
    description: '饮品、食品、包装、硬件和质感产品主图。',
    workflow: '适合为产品方案生成高级摄影感主视觉。',
    icon: 'cube',
    iconClass: 'bg-orange-50 text-orange-600 dark:bg-orange-500/10 dark:text-orange-300',
    badgeClass: 'bg-orange-50 text-orange-700 dark:bg-orange-500/10 dark:text-orange-200',
    fields: simpleFields,
    defaults: {
      topic: '高端乌龙茶饮料包装',
      mustText: 'Aurora Oolong',
    },
    quickActions: [
      { label: '更高级', prompt: '使用商业摄影棚光线、微距材质、高级留白和真实阴影。' },
      { label: '更适合电商', prompt: '产品居中，卖点标签清晰，适合商品详情页首图。' },
    ],
    buildPrompt: values => buildSimplePrompt('产品摄影主图', textValue(values, 'topic'), textValue(values, 'mustText'), [
      '画面：中心产品清晰，材质、反光、包装文字和背景氛围精致。',
      '风格：真实商业摄影，不要塑料 CGI 感，不要假品牌 logo。',
    ]),
  },
  {
    id: 'brandKit',
    name: '品牌 Logo/套件',
    badge: '品牌',
    description: 'Logo 概念、色板、字体、图标和品牌应用板。',
    workflow: '适合快速探索品牌视觉方向，生成一张完整 identity board。',
    icon: 'badge',
    iconClass: 'bg-teal-50 text-teal-600 dark:bg-teal-500/10 dark:text-teal-300',
    badgeClass: 'bg-teal-50 text-teal-700 dark:bg-teal-500/10 dark:text-teal-200',
    fields: simpleFields,
    defaults: {
      topic: 'AI 研究助理品牌',
      mustText: 'Lumen Research',
    },
    quickActions: [
      { label: '更像大厂品牌板', prompt: '加入 Logo、安全区、色板、字体、图标和应用 mockup。' },
      { label: '更年轻', prompt: '更轻盈、有亲和力，但保持专业可信。' },
    ],
    buildPrompt: values => buildSimplePrompt('品牌视觉套件', textValue(values, 'topic'), textValue(values, 'mustText'), [
      '布局：Logo 概念、配色、字体、图标、名片或 App 图标应用，排列成设计系统展示板。',
      '要求：不要使用真实商标；中文字母清晰；像专业品牌提案第一页。',
    ]),
  },
  {
    id: 'sceneStaging',
    name: '换背景/场景化',
    badge: '编辑',
    description: '把已有图放入海报、展台、地铁灯箱、户外广告等真实场景。',
    workflow: '上传参考图后使用效果最好；也可以直接描述要做的展示场景。',
    icon: 'upload',
    iconClass: 'bg-lime-50 text-lime-700 dark:bg-lime-500/10 dark:text-lime-300',
    badgeClass: 'bg-lime-50 text-lime-700 dark:bg-lime-500/10 dark:text-lime-200',
    fields: simpleFields,
    defaults: {
      topic: '把海报放进地铁灯箱广告位',
      mustText: '保持原图主要文字',
    },
    quickActions: [
      { label: '地铁灯箱', prompt: '变成真实地铁站灯箱，玻璃反光、金属边框、远处行人虚化。' },
      { label: '发布会大屏', prompt: '变成发布会现场大屏展示，舞台灯光克制，观众轻微虚化。' },
    ],
    buildPrompt: values => buildSimplePrompt('场景化展示图', textValue(values, 'topic'), textValue(values, 'mustText'), [
      '如果提供了参考图，请保留参考图主体、文字和比例，把它自然放入真实展示场景。',
      '要求：透视准确，光照统一，参考图内容仍然清晰可读。',
    ]),
  },
  {
    id: 'inkChinese',
    name: '国风水墨',
    badge: '风格',
    description: '中文文化、山水、节气、传统审美和新中式视觉。',
    workflow: '适合文化主题封面、节气海报、国风品牌图。',
    icon: 'cloud',
    iconClass: 'bg-slate-50 text-slate-700 dark:bg-slate-500/10 dark:text-slate-200',
    badgeClass: 'bg-slate-50 text-slate-700 dark:bg-slate-500/10 dark:text-slate-200',
    fields: simpleFields,
    defaults: {
      topic: '东方 AI 智能体',
      mustText: '山海有灵',
    },
    quickActions: [
      { label: '更现代', prompt: '融合现代科技细线和留白，不要复古过头。' },
      { label: '更水墨', prompt: '增强宣纸、水墨晕染、远山层次和克制中文排版。' },
    ],
    buildPrompt: values => buildSimplePrompt('国风水墨视觉', textValue(values, 'topic'), textValue(values, 'mustText'), [
      '风格：新中式、水墨留白、细腻层次，中文排版克制，适合文化类内容。',
      '要求：不要廉价古风模板，不要过度金色，不要乱码书法。',
    ]),
  },
  {
    id: 'cyberpunkHud',
    name: '赛博朋克/游戏 HUD',
    badge: '游戏',
    description: '未来城市、游戏界面、HUD、机甲和科技感视觉。',
    workflow: '适合游戏概念图、科技封面、未来感内容包装。',
    icon: 'bolt',
    iconClass: 'bg-cyan-50 text-cyan-700 dark:bg-cyan-500/10 dark:text-cyan-200',
    badgeClass: 'bg-cyan-50 text-cyan-700 dark:bg-cyan-500/10 dark:text-cyan-200',
    fields: simpleFields,
    defaults: {
      topic: 'AI 模型竞技场',
      mustText: 'MODEL ARENA',
    },
    quickActions: [
      { label: '更像游戏界面', prompt: '加入血条、地图、任务提示、技能按钮等 HUD 元素，但保持可读。' },
      { label: '更电影感', prompt: '减少 UI 元素，增强霓虹城市、雨夜反光和电影构图。' },
    ],
    buildPrompt: values => buildSimplePrompt('赛博朋克游戏 HUD 视觉', textValue(values, 'topic'), textValue(values, 'mustText'), [
      '画面：未来城市或竞技场主视觉，叠加可信 HUD 信息层，霓虹但不混乱。',
      '要求：主体清晰，UI 标签可读，避免无意义乱码和过度紫蓝渐变。',
    ]),
  },
]

const selectedTemplate = computed(() => {
  return promptTemplates.find(template => template.id === selectedTemplateId.value) || null
})

const templatePromptPreview = computed(() => {
  const template = selectedTemplate.value
  if (!template) return ''
  return template.buildPrompt(templateForm)
})

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

function resultMeta(item: ImageResult): string {
  const mode = item.mode === 'edit' ? t('imageGeneration.editedImage') : t('imageGeneration.generatedImage')
  return `${mode} · ${t('imageGeneration.completedIn', { seconds: formatSeconds(item.durationMs) })}`
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

function resizePromptInput() {
  const input = promptInput.value
  if (!input) return
  input.style.height = 'auto'
  input.style.height = `${Math.min(input.scrollHeight, 160)}px`
}

function focusPrompt() {
  promptInput.value?.focus()
  resizePromptInput()
}

function resetTemplateForm(template: PromptTemplate) {
  for (const key of Object.keys(templateForm)) {
    delete templateForm[key]
  }
  for (const field of template.fields) {
    templateForm[field.key] = template.defaults[field.key] || field.options?.[0] || ''
  }
}

function applyTemplatePrompt(focus = true) {
  const template = selectedTemplate.value
  if (!template) return
  prompt.value = templatePromptPreview.value
  void nextTick(() => {
    resizePromptInput()
    if (focus) {
      focusPrompt()
    }
  })
}

function selectTemplate(id: TemplateId) {
  const template = promptTemplates.find(item => item.id === id)
  if (!template) return
  selectedTemplateId.value = id
  resetTemplateForm(template)
  applyTemplatePrompt(false)
}

function clearTemplateSelection() {
  selectedTemplateId.value = null
  for (const key of Object.keys(templateForm)) {
    delete templateForm[key]
  }
}

function appendPromptLine(line: string) {
  const trimmed = line.trim()
  if (!trimmed) return
  prompt.value = prompt.value.trim()
    ? `${prompt.value.trim()}\n\n${trimmed}`
    : trimmed
  void nextTick(focusPrompt)
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

async function buildResult(
  promptText: string,
  image: GeneratedImage,
  durationMs: number,
  mode: ImageMode,
): Promise<ImageResult | null> {
  const url = imageToDataUrl(image)
  if (!url) return null
  const id = `${Date.now()}-${Math.random().toString(36).slice(2)}`
  let sourceBlob: Blob | null = null
  try {
    sourceBlob = await imageToBlob(image, generationAbortController?.signal)
  } catch {
    sourceBlob = null
  }

  return {
    id,
    prompt: promptText,
    url,
    durationMs,
    fileName: `image-${id}.png`,
    mode,
    sourceBlob,
  }
}

function imageErrorMessage(err: unknown, fallback: string): string {
  if (
    axios.isAxiosError(err)
    && (err.code === 'ECONNABORTED' || err.message.toLowerCase().includes('timeout'))
  ) {
    return t('imageGeneration.timeoutMessage')
  }
  if (
    axios.isAxiosError(err)
    && (err.code === 'ERR_NETWORK' || err.message.toLowerCase() === 'network error')
  ) {
    return t('imageGeneration.networkErrorMessage')
  }

  const openAIMessage = (err as { response?: { data?: { error?: { message?: string } } } })?.response?.data?.error?.message
  return openAIMessage || extractApiErrorMessage(err, fallback)
}

function revokeActiveSourceUrl() {
  const source = activeSource.value
  if (source?.revokeUrl) {
    URL.revokeObjectURL(source.previewUrl)
  }
}

function setActiveSource(source: ActiveSource) {
  revokeActiveSourceUrl()
  activeSource.value = source
  sourcePreviewFailed.value = false
  maskMode.value = false
  maskDirty.value = false
}

function setActiveSourceFromBlob(blob: Blob, fileName: string, label: string, origin: SourceOrigin) {
  setActiveSource({
    blob,
    fileName,
    label,
    origin,
    previewUrl: URL.createObjectURL(blob),
    revokeUrl: true,
  })
}

function clearActiveSource() {
  revokeActiveSourceUrl()
  activeSource.value = null
  sourcePreviewFailed.value = false
  maskMode.value = false
  maskDirty.value = false
}

function handleActiveSourcePreviewLoad() {
  sourcePreviewFailed.value = false
}

function handleActiveSourcePreviewError() {
  sourcePreviewFailed.value = true
}

function triggerUpload() {
  fileInput.value?.click()
}

function handleUpload(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = ''
  if (!file) return
  if (!file.type.startsWith('image/')) {
    errorMessage.value = t('imageGeneration.uploadFailed')
    return
  }
  errorMessage.value = ''
  setActiveSourceFromBlob(file, file.name || 'upload.png', file.name || t('imageGeneration.uploadedImage'), 'upload')
  void nextTick(focusPrompt)
}

function selectResultForEdit(item: ImageResult) {
  if (!item.sourceBlob) {
    errorMessage.value = t('imageGeneration.sourceLoadFailed')
    return
  }
  errorMessage.value = ''
  setActiveSourceFromBlob(item.sourceBlob, item.fileName, t('imageGeneration.latestResult'), 'result')
  void nextTick(focusPrompt)
}

function loadImage(src: string): Promise<HTMLImageElement> {
  return new Promise((resolve, reject) => {
    const image = new Image()
    image.onload = () => resolve(image)
    image.onerror = () => reject(new Error('failed to load image'))
    image.src = src
  })
}

async function prepareMaskCanvas() {
  const source = activeSource.value
  const preview = previewCanvas.value
  const mask = maskCanvas.value
  if (!source || !preview || !mask) return

  try {
    const image = await loadImage(source.previewUrl)
    const width = Math.max(1, image.naturalWidth || image.width)
    const height = Math.max(1, image.naturalHeight || image.height)
    preview.width = width
    preview.height = height
    mask.width = width
    mask.height = height

    const previewCtx = preview.getContext('2d')
    const maskCtx = mask.getContext('2d')
    if (!previewCtx || !maskCtx) return
    previewCtx.clearRect(0, 0, width, height)
    previewCtx.drawImage(image, 0, 0, width, height)
    maskCtx.clearRect(0, 0, width, height)
    maskDirty.value = false
  } catch {
    errorMessage.value = t('imageGeneration.sourceLoadFailed')
    maskMode.value = false
  }
}

function toggleMaskMode() {
  maskMode.value = !maskMode.value
}

function canvasPoint(event: PointerEvent): Point | null {
  const canvas = previewCanvas.value
  if (!canvas) return null
  const rect = canvas.getBoundingClientRect()
  if (rect.width <= 0 || rect.height <= 0) return null
  return {
    x: ((event.clientX - rect.left) / rect.width) * canvas.width,
    y: ((event.clientY - rect.top) / rect.height) * canvas.height,
  }
}

function drawStroke(from: Point, to: Point) {
  const preview = previewCanvas.value
  const mask = maskCanvas.value
  const previewCtx = preview?.getContext('2d')
  const maskCtx = mask?.getContext('2d')
  if (!preview || !mask || !previewCtx || !maskCtx) return

  const width = brushSize.value
  for (const ctx of [maskCtx, previewCtx]) {
    ctx.save()
    ctx.lineCap = 'round'
    ctx.lineJoin = 'round'
    ctx.lineWidth = width
    ctx.beginPath()
    ctx.moveTo(from.x, from.y)
    ctx.lineTo(to.x, to.y)
    if (ctx === maskCtx) {
      ctx.strokeStyle = '#ffffff'
    } else {
      ctx.strokeStyle = 'rgba(20, 184, 166, 0.5)'
    }
    ctx.stroke()
    ctx.restore()
  }
  maskDirty.value = true
}

function startPaint(event: PointerEvent) {
  if (!maskMode.value) return
  const point = canvasPoint(event)
  if (!point) return
  painting = true
  lastPoint = point
  ;(event.currentTarget as HTMLElement).setPointerCapture?.(event.pointerId)
  drawStroke(point, point)
}

function paint(event: PointerEvent) {
  if (!painting || !lastPoint || !maskMode.value) return
  const point = canvasPoint(event)
  if (!point) return
  drawStroke(lastPoint, point)
  lastPoint = point
}

function stopPaint(event?: PointerEvent) {
  if (event?.currentTarget) {
    ;(event.currentTarget as HTMLElement).releasePointerCapture?.(event.pointerId)
  }
  painting = false
  lastPoint = null
}

async function clearMask() {
  await prepareMaskCanvas()
}

function canvasToBlob(canvas: HTMLCanvasElement, type = 'image/png'): Promise<Blob | null> {
  return new Promise((resolve) => {
    canvas.toBlob((blob) => resolve(blob), type)
  })
}

async function exportMaskBlob(): Promise<Blob | null> {
  const sourceMask = maskCanvas.value
  if (!sourceMask || !maskDirty.value) return null

  const output = document.createElement('canvas')
  output.width = sourceMask.width
  output.height = sourceMask.height
  const ctx = output.getContext('2d')
  if (!ctx) return null

  ctx.fillStyle = '#ffffff'
  ctx.fillRect(0, 0, output.width, output.height)
  ctx.globalCompositeOperation = 'destination-out'
  ctx.drawImage(sourceMask, 0, 0)
  return canvasToBlob(output)
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
    const source = activeSource.value
    const mask = source && maskMode.value ? await exportMaskBlob() : null
    const result = source
      ? await editImage(key, {
          source: source.blob,
          sourceFileName: source.fileName,
          mask,
          prompt: promptText,
          signal: generationAbortController.signal,
        })
      : await generateImage(key, promptText, generationAbortController.signal)

    const generated = (await Promise.all((result.response.data || [])
      .map((image) => buildResult(promptText, image, result.durationMs, source ? 'edit' : 'generate'))))
      .filter((item): item is ImageResult => item !== null)

    if (generated.length === 0) {
      errorMessage.value = t('imageGeneration.emptyResponse')
      return
    }

    results.value.push(...generated)
    const latestEditable = generated.find((item) => item.sourceBlob)
    if (latestEditable?.sourceBlob) {
      setActiveSourceFromBlob(latestEditable.sourceBlob, latestEditable.fileName, t('imageGeneration.latestResult'), 'result')
    }
    prompt.value = ''
    await scrollToBottom()
  } catch (err: unknown) {
    if (axios.isCancel(err)) return
    errorMessage.value = imageErrorMessage(err, activeSource.value ? t('imageGeneration.editFailed') : t('imageGeneration.generateFailed'))
  } finally {
    generating.value = false
    stopTimer()
  }
}

watch(maskMode, async (enabled) => {
  if (enabled) {
    await nextTick()
    await prepareMaskCanvas()
  } else {
    stopPaint()
  }
})

watch(templateForm, () => {
  const template = selectedTemplate.value
  if (template) {
    prompt.value = template.buildPrompt(templateForm)
  }
}, { deep: true })

watch(prompt, () => {
  void nextTick(resizePromptInput)
})

onMounted(loadDefaultKey)

onBeforeUnmount(() => {
  keyAbortController?.abort()
  generationAbortController?.abort()
  stopTimer()
  revokeActiveSourceUrl()
})
</script>
