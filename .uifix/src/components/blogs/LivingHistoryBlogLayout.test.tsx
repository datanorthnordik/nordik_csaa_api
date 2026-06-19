import { render, screen } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { BlogDetailResponse } from '../../api/blogsApi'
import {
  LivingHistoryBlogLayout,
  blogHasAnimations,
  blogHasPostAnimationContent,
} from './LivingHistoryBlogLayout'

const blog: BlogDetailResponse = {
  id: 4,
  publish_date: '2026-05-01T00:00:00Z',
  heading: 'The Tree Planting',
  description: 'Card summary',
  cover_image_url: '',
  cover_image_object_key: 'blogs/4/cover.jpg',
  cover_image_fetch_url: '/api/blogs/4/cover/content',
  created_at: '2026-06-19T00:00:00Z',
  updated_at: '2026-06-19T00:00:00Z',
  blog_detail: {
    sections: [
      {
        id: 1,
        section_name: 'Video',
        section_type: 'video',
        sort_order: 0,
        is_enabled: true,
        video: {
          youtube_url: 'https://www.youtube.com/watch?v=PQNA9vqP42g',
          caption: '',
        },
      },
      {
        id: 2,
        section_name: 'Introduction',
        section_type: 'typography',
        sort_order: 1,
        is_enabled: true,
        typography: {
          html_content: '<p>Introduction copy</p>',
          text_content: 'Introduction copy',
        },
      },
      {
        id: 3,
        section_name: 'Timeline',
        section_type: 'animation',
        sort_order: 2,
        is_enabled: true,
        animation: {
          navigation: 'vertical',
          image_position: 'left',
          items: [
            {
              id: 31,
              sort_order: 0,
              heading: 'The First Root',
              sub_heading: '1981',
              description: 'Timeline copy',
              image: {
                file_url: '/api/blogs/4/sections/3/items/31/image/content',
                fetch_url: '/api/blogs/4/sections/3/items/31/image/content',
                storage_uri: '',
                gcp_object_key: 'blogs/4/root.jpg',
              },
            },
          ],
        },
      },
      {
        id: 4,
        section_name: 'Living Memory',
        section_type: 'animation',
        sort_order: 3,
        is_enabled: true,
        animation: {
          navigation: 'horizontal',
          image_position: 'right',
          items: [
            {
              id: 41,
              sort_order: 0,
              heading: 'Living Memory',
              sub_heading: 'The pine today',
              description: 'First state',
              image: {
                file_url: '/api/blogs/4/sections/4/items/41/image/content',
                fetch_url: '/api/blogs/4/sections/4/items/41/image/content',
                storage_uri: '',
                gcp_object_key: 'blogs/4/first.png',
              },
            },
            {
              id: 42,
              sort_order: 1,
              heading: 'Still Standing Today',
              sub_heading: 'Twin pines',
              description: 'Final state',
              image: {
                file_url: '/api/blogs/4/sections/4/items/42/image/content',
                fetch_url: '/api/blogs/4/sections/4/items/42/image/content',
                storage_uri: '',
                gcp_object_key: 'blogs/4/final.png',
              },
            },
          ],
        },
      },
      {
        id: 5,
        section_name: 'Memorial action',
        section_type: 'action',
        sort_order: 4,
        is_enabled: true,
        action: {
          text: 'Healing & Memorials',
          action_type: 'link',
          target_url: '/our-story/healing-memorials',
        },
      },
    ],
  },
}

describe('LivingHistoryBlogLayout', () => {
  beforeEach(() => {
    Object.defineProperty(window, 'IntersectionObserver', {
      writable: true,
      value: class {
        observe() {}
        disconnect() {}
      },
    })
    vi.stubGlobal('requestAnimationFrame', vi.fn(() => 1))
    vi.stubGlobal('cancelAnimationFrame', vi.fn())
  })

  it('keeps video and typography in the article phase', () => {
    render(
      <LivingHistoryBlogLayout
        blog={blog}
        phase="article"
        onOpenVideo={vi.fn()}
      />,
    )

    expect(screen.getByRole('button', { name: 'Play Video' })).toBeTruthy()
    expect(screen.getByText('Introduction copy')).toBeTruthy()
    expect(screen.queryByText('The First Root')).toBeNull()
  })

  it('renders animations outside the article and folds the trailing action into the final reveal', () => {
    render(
      <LivingHistoryBlogLayout
        blog={blog}
        phase="animations"
        onOpenVideo={vi.fn()}
      />,
    )

    expect(screen.getByText('The First Root')).toBeTruthy()
    expect(screen.getByText('Living Memory')).toBeTruthy()
    expect(screen.getByText('Still Standing Today')).toBeTruthy()
    expect(
      screen.getByRole('link', { name: 'Healing & Memorials' }).getAttribute('href'),
    ).toBe('/our-story/healing-memorials')
    expect(blogHasAnimations(blog)).toBe(true)
    expect(blogHasPostAnimationContent(blog)).toBe(false)
  })
})
