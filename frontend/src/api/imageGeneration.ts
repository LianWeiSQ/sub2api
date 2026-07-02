import axios from 'axios'
import keysAPI from './keys'
import type { ApiKey } from '@/types'

const IMAGE_GENERATION_ENDPOINT = '/v1/images/generations'
const IMAGE_EDIT_ENDPOINT = '/v1/images/edits'
const IMAGE_GENERATION_TIMEOUT_MS = 600_000
const IMAGE_MODEL = 'gpt-image-2'
const IMAGE_SIZE = '1024x1024'
const IMAGE_QUALITY = 'high'

export interface GeneratedImage {
  b64_json?: string
  url?: string
  revised_prompt?: string
}

export interface ImageGenerationUsage {
  input_tokens?: number
  output_tokens?: number
  total_tokens?: number
}

export interface ImageGenerationResponse {
  created?: number
  data?: GeneratedImage[]
  model?: string
  quality?: string
  size?: string
  usage?: ImageGenerationUsage
}

export interface ImageGenerationResult {
  response: ImageGenerationResponse
  durationMs: number
}

export async function getDefaultImageApiKey(signal?: AbortSignal): Promise<ApiKey | null> {
  const response = await keysAPI.list(1, 50, { status: 'active' }, { signal })
  return response.items.find((key) => (
    key.status === 'active'
    && Boolean(key.key)
    && key.group?.platform === 'openai'
  )) ?? null
}

export async function generateImage(
  apiKey: string,
  prompt: string,
  signal?: AbortSignal,
): Promise<ImageGenerationResult> {
  const startedAt = performance.now()
  const { data } = await axios.post<ImageGenerationResponse>(
    IMAGE_GENERATION_ENDPOINT,
    {
      model: IMAGE_MODEL,
      prompt,
      size: IMAGE_SIZE,
      quality: IMAGE_QUALITY,
      n: 1,
    },
    {
      headers: {
        Authorization: `Bearer ${apiKey}`,
        'Content-Type': 'application/json',
      },
      signal,
      timeout: IMAGE_GENERATION_TIMEOUT_MS,
    },
  )

  return {
    response: data,
    durationMs: Math.round(performance.now() - startedAt),
  }
}

export interface EditImageOptions {
  source: Blob
  sourceFileName?: string
  mask?: Blob | null
  prompt: string
  signal?: AbortSignal
}

export async function editImage(
  apiKey: string,
  options: EditImageOptions,
): Promise<ImageGenerationResult> {
  const startedAt = performance.now()
  const formData = new FormData()
  formData.append('model', IMAGE_MODEL)
  formData.append('prompt', options.prompt)
  formData.append('size', IMAGE_SIZE)
  formData.append('quality', IMAGE_QUALITY)
  formData.append('input_fidelity', 'high')
  formData.append('n', '1')
  formData.append('image', options.source, options.sourceFileName || 'source.png')
  if (options.mask) {
    formData.append('mask', options.mask, 'mask.png')
  }

  const { data } = await axios.post<ImageGenerationResponse>(
    IMAGE_EDIT_ENDPOINT,
    formData,
    {
      headers: {
        Authorization: `Bearer ${apiKey}`,
      },
      signal: options.signal,
      timeout: IMAGE_GENERATION_TIMEOUT_MS,
    },
  )

  return {
    response: data,
    durationMs: Math.round(performance.now() - startedAt),
  }
}

export function imageToDataUrl(image: GeneratedImage): string {
  if (image.url) return image.url
  if (image.b64_json) return `data:image/png;base64,${image.b64_json}`
  return ''
}

export function dataUrlToBlob(dataUrl: string): Blob | null {
  const match = dataUrl.match(/^data:([^;,]+)?(;base64)?,(.*)$/)
  if (!match) return null

  const mimeType = match[1] || 'image/png'
  const isBase64 = Boolean(match[2])
  const payload = match[3] || ''

  try {
    const binary = isBase64 ? atob(payload) : decodeURIComponent(payload)
    const bytes = new Uint8Array(binary.length)
    for (let i = 0; i < binary.length; i += 1) {
      bytes[i] = binary.charCodeAt(i)
    }
    return new Blob([bytes], { type: mimeType })
  } catch {
    return null
  }
}

export async function imageToBlob(image: GeneratedImage, signal?: AbortSignal): Promise<Blob | null> {
  if (image.b64_json) {
    return dataUrlToBlob(`data:image/png;base64,${image.b64_json}`)
  }
  if (!image.url) return null

  const response = await fetch(image.url, { signal })
  if (!response.ok) return null
  return response.blob()
}
