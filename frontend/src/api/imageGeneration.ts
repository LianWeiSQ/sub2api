import axios from 'axios'
import keysAPI from './keys'
import type { ApiKey } from '@/types'

const IMAGE_GENERATION_ENDPOINT = '/v1/images/generations'
const IMAGE_GENERATION_TIMEOUT_MS = 600_000

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
  return response.items.find((key) => key.status === 'active' && Boolean(key.key)) ?? null
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
      model: 'gpt-image-2',
      prompt,
      size: '1024x1024',
      quality: 'high',
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

export function imageToDataUrl(image: GeneratedImage): string {
  if (image.url) return image.url
  if (image.b64_json) return `data:image/png;base64,${image.b64_json}`
  return ''
}
