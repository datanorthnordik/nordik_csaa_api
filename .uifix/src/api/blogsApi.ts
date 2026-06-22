import { API_BASE_URL, API_ROUTES } from '../constants/api'
import { apiClient } from './apiClient'

export type BlogActionType = 'link' | 'video'
export type BlogAnimationNavigation = 'vertical' | 'horizontal'
export type BlogAnimationImagePosition = 'left' | 'right'

export type BlogSectionAsset = {
  file_url: string
  fetch_url: string
  storage_uri: string
  gcp_object_key: string
}

export type BlogHeadingSection = {
  heading_text: string
  underline_enabled: boolean
}

export type BlogImageSection = {
  asset?: BlogSectionAsset | null
  caption: string
}

export type BlogTypographySection = {
  html_content: string
  text_content: string
}

export type BlogActionSection = {
  text: string
  action_type: BlogActionType
  target_url: string
}

export type BlogVideoSection = {
  youtube_url: string
  caption: string
}

export type BlogAnimationItem = {
  id: number
  sort_order: number
  heading: string
  sub_heading: string
  description: string
  image?: BlogSectionAsset | null
}

export type BlogAnimationSection = {
  navigation: BlogAnimationNavigation
  image_position: BlogAnimationImagePosition
  items: BlogAnimationItem[]
}

export type BlogSection = {
  id: number
  section_name: string
  section_type: 'heading' | 'image' | 'typography' | 'action' | 'video' | 'animation' | string
  sort_order: number
  is_enabled: boolean
  heading?: BlogHeadingSection | null
  image?: BlogImageSection | null
  typography?: BlogTypographySection | null
  action?: BlogActionSection | null
  video?: BlogVideoSection | null
  animation?: BlogAnimationSection | null
}

export type BlogListItem = {
  id: number
  publish_date: string
  heading: string
  description: string
  cover_image_url: string
  cover_image_object_key: string
  cover_image_fetch_url: string
  updated_at: string
  created_at: string
}

export type BlogDetailResponse = {
  id: number
  publish_date: string
  heading: string
  description: string
  cover_image_url: string
  cover_image_object_key: string
  cover_image_fetch_url: string
  created_at: string
  updated_at: string
  blog_detail?: {
    sections: BlogSection[]
  } | null
}

export function resolveBlogAssetUrl(value: string | null | undefined) {
  const trimmed = value?.trim() ?? ''
  if (!trimmed) {
    return ''
  }
  if (/^[a-z][a-z0-9+.-]*:/i.test(trimmed) || trimmed.startsWith('//')) {
    return trimmed
  }
  try {
    return new URL(trimmed, `${API_BASE_URL}/`).toString()
  } catch {
    return trimmed
  }
}

export const blogsApi = {
  async listBlogs() {
    const response = await apiClient.get<{ items: BlogListItem[] | null }>(API_ROUTES.blogs, {
      skipAuth: true,
      skipErrorToast: true,
      params: {
        sort_by: 'publish_date',
        sort_order: 'desc',
      },
    })

    return Array.isArray(response.data.items) ? response.data.items : []
  },

  async getBlog(id: number) {
    const response = await apiClient.get<BlogDetailResponse>(API_ROUTES.blogById(id), {
      skipAuth: true,
      skipErrorToast: true,
    })
    return response.data
  },
}
