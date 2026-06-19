import { useEffect, useRef, useState } from 'react'
import type { BlogDetailResponse, BlogSection } from '../../api/blogsApi'
import { resolveBlogAssetUrl } from '../../api/blogsApi'
import styles from '../../pages/LivingHistoryHubPage.module.css'

type LivingHistoryBlogContentProps = {
  blog: BlogDetailResponse
  onOpenVideo: (videoUrl: string) => void
}

export function LivingHistoryBlogContent({
  blog,
  onOpenVideo,
}: LivingHistoryBlogContentProps) {
  const sections = (blog.blog_detail?.sections ?? [])
    .filter((section) => section.is_enabled)
    .slice()
    .sort((left, right) => left.sort_order - right.sort_order)

  const normalizedHeading = blog.heading.trim().toLowerCase()
  const visibleSections = sections.filter((section, index) => {
    if (
      index === 0 &&
      section.section_type === 'heading' &&
      section.heading?.heading_text.trim().toLowerCase() === normalizedHeading
    ) {
      return false
    }
    return true
  })

  return (
    <>
      {visibleSections.map((section) => (
        <BlogSectionRenderer
          key={section.id}
          section={section}
          onOpenVideo={onOpenVideo}
        />
      ))}
    </>
  )
}

function BlogSectionRenderer({
  section,
  onOpenVideo,
}: {
  section: BlogSection
  onOpenVideo: (videoUrl: string) => void
}) {
  switch (section.section_type) {
    case 'heading':
      return section.heading ? (
        <section className={styles.blogBlock}>
          <h1 className={styles.blogArticleTitle}>{section.heading.heading_text}</h1>
          {section.heading.underline_enabled && (
            <div className={styles.featureRule} aria-hidden="true" />
          )}
        </section>
      ) : null

    case 'image':
      return section.image?.asset ? (
        <figure className={styles.blogHero}>
          <img
            src={resolveBlogAssetUrl(section.image.asset.fetch_url || section.image.asset.file_url)}
            alt={section.section_name || 'Blog image'}
          />
          {section.image.caption && <figcaption>{section.image.caption}</figcaption>}
        </figure>
      ) : null

    case 'typography':
      return section.typography ? (
        <section
          className={styles.blogRichText}
          dangerouslySetInnerHTML={{ __html: section.typography.html_content }}
        />
      ) : null

    case 'action':
      return section.action ? (
        <section className={styles.blogBlock}>
          {section.action.action_type === 'video' ? (
            <button
              type="button"
              className={styles.revealButton}
              onClick={() => onOpenVideo(section.action!.target_url)}
            >
              {section.action.text}
            </button>
          ) : (
            <a
              href={section.action.target_url}
              className={styles.revealButton}
            >
              {section.action.text}
            </a>
          )}
        </section>
      ) : null

    case 'video':
      return section.video ? (
        <section className={styles.blogBlock}>
          <button
            type="button"
            className={styles.blogVideoFacade}
            style={{ backgroundImage: `url(${thumbnailUrl(section.video.youtube_url)})` }}
            onClick={() => onOpenVideo(section.video!.youtube_url)}
            aria-label={section.section_name || 'Play video'}
          >
            <span className={styles.videoPlay} aria-hidden="true">
              <svg viewBox="0 0 24 24" width="26" height="26">
                <path d="M8 5v14l11-7z" fill="currentColor" />
              </svg>
            </span>
          </button>
          {section.video.caption && <p className={styles.blogByline}>{section.video.caption}</p>}
        </section>
      ) : null

    case 'animation':
      return section.animation ? (
        section.animation.navigation === 'vertical' ? (
          <VerticalAnimationSection section={section} />
        ) : (
          <HorizontalAnimationSection section={section} />
        )
      ) : null

    default:
      return null
  }
}

function VerticalAnimationSection({ section }: { section: BlogSection }) {
  const items = (section.animation?.items ?? [])
    .slice()
    .sort((left, right) => left.sort_order - right.sort_order)
  const blockRefs = useRef<(HTMLDivElement | null)[]>([])
  const [activeIndex, setActiveIndex] = useState(0)
  const activeItem = items[activeIndex] ?? items[0]
  const imageOnRight = section.animation?.image_position === 'right'

  useEffect(() => {
    const blocks = blockRefs.current.filter(Boolean) as HTMLDivElement[]
    if (!blocks.length) {
      return
    }

    const observer = new IntersectionObserver(
      (entries) => {
        entries.forEach((entry) => {
          if (!entry.isIntersecting) {
            return
          }
          const index = blockRefs.current.indexOf(entry.target as HTMLDivElement)
          if (index >= 0) {
            setActiveIndex(index)
          }
        })
      },
      { rootMargin: '-45% 0px -45% 0px', threshold: 0 },
    )

    blocks.forEach((block) => observer.observe(block))
    return () => observer.disconnect()
  }, [items.length])

  if (!activeItem) {
    return null
  }

  return (
    <section className={styles.feature}>
      <div className={styles.featureInner}>
        <div
          className={styles.featureSticky}
          style={{ order: imageOnRight ? 2 : 1 }}
        >
          <div className={styles.featureImageWrap}>
            {items.map((item, index) => {
              const imageUrl = resolveBlogAssetUrl(item.image?.fetch_url || item.image?.file_url)
              return imageUrl ? (
                <img
                  key={item.id}
                  src={imageUrl}
                  alt={index === activeIndex ? item.heading : ''}
                  className={styles.featureImage}
                  style={{ opacity: index === activeIndex ? 1 : 0 }}
                  aria-hidden={index !== activeIndex}
                />
              ) : null
            })}
          </div>
        </div>

        <div
          className={styles.featureBlocks}
          style={{ order: imageOnRight ? 1 : 2 }}
        >
          {items.map((item, index) => (
            <div
              key={item.id}
              ref={(node) => {
                blockRefs.current[index] = node
              }}
              className={styles.featureBlock}
            >
              <h2 className={styles.featureTitle}>{item.heading}</h2>
              <div className={styles.featureRule} aria-hidden="true" />
              <p className={styles.featureSubhead}>{item.sub_heading}</p>
              <p className={styles.featureBody}>{item.description}</p>
            </div>
          ))}
        </div>
      </div>
    </section>
  )
}

function HorizontalAnimationSection({ section }: { section: BlogSection }) {
  const items = (section.animation?.items ?? [])
    .slice()
    .sort((left, right) => left.sort_order - right.sort_order)
  const [activeIndex, setActiveIndex] = useState(0)
  const activeItem = items[activeIndex] ?? items[0]
  const imageOnRight = section.animation?.image_position === 'right'
  const imageUrl = resolveBlogAssetUrl(activeItem?.image?.fetch_url || activeItem?.image?.file_url)

  if (!activeItem) {
    return null
  }

  return (
    <section className={styles.editions}>
      <div className={styles.editionsInner}>
        <div
          className={`${styles.horizontalFeature} ${
            imageOnRight ? styles.horizontalFeatureImageRight : ''
          }`}
        >
          <div className={styles.horizontalFeatureMedia}>
            {imageUrl ? (
              <img src={imageUrl} alt={activeItem.heading} className={styles.heroPreviewImage} />
            ) : null}
          </div>

          <div className={styles.horizontalFeatureContent}>
            <h2 className={styles.featureTitle}>{activeItem.heading}</h2>
            <div className={styles.featureRule} aria-hidden="true" />
            <p className={styles.featureSubhead}>{activeItem.sub_heading}</p>
            <p className={styles.featureBody}>{activeItem.description}</p>
          </div>
        </div>

        <div className={styles.edShelf}>
          <p className={styles.edShelfLabel}>{section.section_name || 'Story moments'}</p>
          <div className={styles.edShelfRow}>
            {items.map((item, index) => (
              <button
                key={item.id}
                type="button"
                className={styles.edCard}
                data-active={index === activeIndex}
                onClick={() => setActiveIndex(index)}
              >
                <span className={styles.edCardNo}>{String(index + 1).padStart(2, '0')}</span>
                <span className={styles.edCardTitle}>{item.heading}</span>
                <span className={styles.edCardPeriod}>{item.sub_heading}</span>
              </button>
            ))}
          </div>
        </div>
      </div>
    </section>
  )
}

function extractYouTubeId(url: string) {
  try {
    const parsed = new URL(url)
    if (parsed.hostname.includes('youtu.be')) {
      return parsed.pathname.split('/').filter(Boolean)[0] ?? ''
    }
    const watchId = parsed.searchParams.get('v')
    if (watchId) {
      return watchId
    }
    const pathMatch = parsed.pathname.match(/\/(?:embed|shorts)\/([^/?]+)/)
    return pathMatch?.[1] ?? ''
  } catch {
    const match = url.match(/(?:v=|youtu\.be\/|embed\/|shorts\/)([A-Za-z0-9_-]+)/)
    return match?.[1] ?? ''
  }
}

function thumbnailUrl(youtubeUrl: string) {
  const youtubeId = extractYouTubeId(youtubeUrl)
  return youtubeId ? `https://i.ytimg.com/vi/${youtubeId}/hqdefault.jpg` : ''
}
