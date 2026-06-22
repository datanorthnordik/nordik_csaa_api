import { describe, expect, it } from 'vitest'
import { API_BASE_URL } from '../constants/api'
import { resolveBlogAssetUrl } from './blogsApi'

describe('resolveBlogAssetUrl', () => {
  it('resolves API content paths against the API host', () => {
    expect(resolveBlogAssetUrl('/api/blogs/4/cover/content')).toBe(
      `${API_BASE_URL}/api/blogs/4/cover/content`,
    )
  })

  it('keeps absolute storage URLs unchanged', () => {
    const url = 'https://storage.googleapis.com/example/blog-cover.jpg'
    expect(resolveBlogAssetUrl(url)).toBe(url)
  })
})
