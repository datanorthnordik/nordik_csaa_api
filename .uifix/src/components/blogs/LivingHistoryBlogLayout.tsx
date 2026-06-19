import { useEffect, useRef, useState } from 'react'
import type {
  BlogActionSection,
  BlogDetailResponse,
  BlogSection,
} from '../../api/blogsApi'
import { resolveBlogAssetUrl } from '../../api/blogsApi'
import styles from '../../pages/LivingHistoryHubPage.module.css'

export type BlogLayoutPhase = 'article' | 'animations' | 'after-animations'
export type BlogLayoutGroup = {
  kind: 'article' | 'animation'
  sections: BlogSection[]
}

type LivingHistoryBlogLayoutProps = {
  blog: BlogDetailResponse
  phase: BlogLayoutPhase
  onOpenVideo: (videoUrl: string) => void
}

export function getBlogLayoutGroups(blog: BlogDetailResponse): BlogLayoutGroup[] {
  const groups: BlogLayoutGroup[] = []
  const sections = getVisibleSections(blog)

  sections.forEach((section, index) => {
    const previous = sections[index - 1]
    const belongsToAnimation =
      section.section_type === 'animation' ||
      (section.section_type === 'action' &&
        previous?.section_type === 'animation' &&
        previous.animation?.navigation === 'horizontal')
    const kind: BlogLayoutGroup['kind'] = belongsToAnimation
      ? 'animation'
      : 'article'
    const current = groups[groups.length - 1]
    if (current?.kind === kind) {
      current.sections.push(section)
    } else {
      groups.push({ kind, sections: [section] })
    }
  })

  return groups
}

export function LivingHistoryBlogGroup({
  group,
  onOpenVideo,
}: {
  group: BlogLayoutGroup
  onOpenVideo: (videoUrl: string) => void
}) {
  if (group.kind === 'article') {
    return <SectionList sections={group.sections} onOpenVideo={onOpenVideo} />
  }

  return (
    <>
      {group.sections.map((section, index) => {
        if (section.section_type === 'action') {
          return null
        }
        const next = group.sections[index + 1]
        return (
          <SectionRenderer
            key={section.id}
            section={section}
            onOpenVideo={onOpenVideo}
            trailingAction={
              section.animation?.navigation === 'horizontal' &&
              next?.section_type === 'action'
                ? next.action
                : null
            }
          />
        )
      })}
    </>
  )
}

export function LivingHistoryBlogLayout({
  blog,
  phase,
  onOpenVideo,
}: LivingHistoryBlogLayoutProps) {
  const sections = getVisibleSections(blog)
  const firstAnimationIndex = sections.findIndex(
    (section) => section.section_type === 'animation',
  )
  const lastAnimationIndex = sections.findLastIndex(
    (section) => section.section_type === 'animation',
  )
  const trailingAction =
    lastAnimationIndex >= 0 &&
    sections[lastAnimationIndex]?.animation?.navigation === 'horizontal' &&
    sections[lastAnimationIndex + 1]?.section_type === 'action'
      ? sections[lastAnimationIndex + 1]
      : null

  if (phase === 'article') {
    const articleSections =
      firstAnimationIndex < 0 ? sections : sections.slice(0, firstAnimationIndex)
    return <SectionList sections={articleSections} onOpenVideo={onOpenVideo} />
  }

  if (phase === 'animations') {
    return (
      <>
        {sections
          .filter((section) => section.section_type === 'animation')
          .map((section) => (
            <SectionRenderer
              key={section.id}
              section={section}
              onOpenVideo={onOpenVideo}
              trailingAction={
                section.id === sections[lastAnimationIndex]?.id
                  ? trailingAction?.action ?? null
                  : null
              }
            />
          ))}
      </>
    )
  }

  if (lastAnimationIndex < 0) {
    return null
  }
  const afterSections = sections
    .slice(lastAnimationIndex + 1)
    .filter((section) => section.id !== trailingAction?.id)
  return <SectionList sections={afterSections} onOpenVideo={onOpenVideo} />
}

export function blogHasAnimations(blog: BlogDetailResponse) {
  return getBlogLayoutGroups(blog).some((group) => group.kind === 'animation')
}

export function blogHasPostAnimationContent(blog: BlogDetailResponse) {
  const sections = getVisibleSections(blog)
  const lastAnimationIndex = sections.findLastIndex(
    (section) => section.section_type === 'animation',
  )
  if (lastAnimationIndex < 0) {
    return false
  }
  const hasInlineTrailingAction =
    sections[lastAnimationIndex]?.animation?.navigation === 'horizontal' &&
    sections[lastAnimationIndex + 1]?.section_type === 'action'
  return sections
    .slice(lastAnimationIndex + (hasInlineTrailingAction ? 2 : 1))
    .some((section) => section.section_type !== 'animation')
}

function getVisibleSections(blog: BlogDetailResponse) {
  const normalizedHeading = blog.heading.trim().toLowerCase()
  return (blog.blog_detail?.sections ?? [])
    .filter((section) => section.is_enabled)
    .slice()
    .sort((left, right) => left.sort_order - right.sort_order)
    .filter(
      (section, index) =>
        !(
          index === 0 &&
          section.section_type === 'heading' &&
          section.heading?.heading_text.trim().toLowerCase() === normalizedHeading
        ),
    )
}

function SectionList({
  sections,
  onOpenVideo,
}: {
  sections: BlogSection[]
  onOpenVideo: (videoUrl: string) => void
}) {
  return (
    <>
      {sections.map((section) => (
        <SectionRenderer
          key={section.id}
          section={section}
          onOpenVideo={onOpenVideo}
        />
      ))}
    </>
  )
}

function SectionRenderer({
  section,
  onOpenVideo,
  trailingAction,
}: {
  section: BlogSection
  onOpenVideo: (videoUrl: string) => void
  trailingAction?: BlogActionSection | null
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
            src={resolveBlogAssetUrl(
              section.image.asset.fetch_url || section.image.asset.file_url,
            )}
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
        <Action action={section.action} onOpenVideo={onOpenVideo} />
      ) : null

    case 'video':
      return section.video ? (
        <InlineVideo
          title={section.section_name || 'Play video'}
          youtubeUrl={section.video.youtube_url}
          caption={section.video.caption}
        />
      ) : null

    case 'animation':
      if (!section.animation) {
        return null
      }
      return section.animation.navigation === 'vertical' ? (
        <VerticalAnimationSection section={section} />
      ) : (
        <HorizontalAnimationSection
          section={section}
          trailingAction={trailingAction}
          onOpenVideo={onOpenVideo}
        />
      )

    default:
      return null
  }
}

function Action({
  action,
  onOpenVideo,
}: {
  action: BlogActionSection
  onOpenVideo: (videoUrl: string) => void
}) {
  return (
    <div className={styles.blogBlock}>
      {action.action_type === 'video' ? (
        <button
          type="button"
          className={styles.revealButton}
          onClick={() => onOpenVideo(action.target_url)}
        >
          {action.text}
        </button>
      ) : (
        <a href={action.target_url} className={styles.revealButton}>
          {action.text}
        </a>
      )}
    </div>
  )
}

function InlineVideo({
  title,
  youtubeUrl,
  caption,
}: {
  title: string
  youtubeUrl: string
  caption: string
}) {
  const [playing, setPlaying] = useState(false)
  const youtubeId = extractYouTubeId(youtubeUrl)
  if (!youtubeId) {
    return null
  }

  return (
    <>
      <div className={styles.blogVideo}>
        {playing ? (
          <iframe
            src={embedUrl(youtubeId)}
            title={title}
            allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture"
            allowFullScreen
          />
        ) : (
          <button
            type="button"
            className={styles.blogVideoFacade}
            style={{ backgroundImage: `url(${thumbnailUrl(youtubeId)})` }}
            onClick={() => setPlaying(true)}
            aria-label={`Play ${title}`}
          >
            <PlayIcon />
          </button>
        )}
      </div>
      {caption && <p className={styles.blogByline}>{caption}</p>}
    </>
  )
}

function VerticalAnimationSection({ section }: { section: BlogSection }) {
  const items = (section.animation?.items ?? [])
    .slice()
    .sort((left, right) => left.sort_order - right.sort_order)
  const blockRefs = useRef<(HTMLDivElement | null)[]>([])
  const [activeIndex, setActiveIndex] = useState(0)
  const imageOnRight = section.animation?.image_position === 'right'

  useEffect(() => {
    const blocks = blockRefs.current.filter(Boolean) as HTMLDivElement[]
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

  if (!items.length) {
    return null
  }
  return (
    <section className={styles.feature}>
      <div className={styles.featureInner}>
        <div className={styles.featureSticky} style={{ order: imageOnRight ? 2 : 1 }}>
          <div className={styles.featureImageWrap}>
            {items.map((item, index) => {
              const imageUrl = resolveBlogAssetUrl(
                item.image?.fetch_url || item.image?.file_url,
              )
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
        <div className={styles.featureBlocks} style={{ order: imageOnRight ? 1 : 2 }}>
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

function HorizontalAnimationSection({
  section,
  trailingAction,
  onOpenVideo,
}: {
  section: BlogSection
  trailingAction?: BlogActionSection | null
  onOpenVideo: (videoUrl: string) => void
}) {
  const items = (section.animation?.items ?? [])
    .slice()
    .sort((left, right) => left.sort_order - right.sort_order)
  const first = items[0]
  const last = items[items.length - 1]
  const slotARef = useRef<HTMLDivElement>(null)
  const slotBRef = useRef<HTMLDivElement>(null)
  const flyRef = useRef<HTMLDivElement>(null)
  const detailRef = useRef<HTMLDivElement>(null)
  const introRef = useRef<HTMLDivElement>(null)
  const firstImageRef = useRef<HTMLImageElement>(null)
  const lastImageRef = useRef<HTMLImageElement>(null)

  useEffect(() => {
    const fly = flyRef.current
    const slotA = slotARef.current
    const slotB = slotBRef.current
    if (!fly || !slotA || !slotB) {
      return
    }
    const lerp = (a: number, b: number, amount: number) => a + (b - a) * amount
    const smoothStep = (a: number, b: number, value: number) => {
      const amount = Math.min(1, Math.max(0, (value - a) / (b - a)))
      return amount * amount * (3 - 2 * amount)
    }
    let frame = 0
    const update = () => {
      frame = 0
      const start = slotA.getBoundingClientRect()
      const end = slotB.getBoundingClientRect()
      const viewportCenter = window.innerHeight / 2
      const startCenter = start.top + start.height / 2
      const endCenter = end.top + end.height / 2
      const span = startCenter - endCenter
      const progress =
        span === 0
          ? 0
          : Math.min(1, Math.max(0, (startCenter - viewportCenter) / span))

      fly.style.left = `${lerp(start.left, end.left, progress)}px`
      fly.style.top = `${lerp(start.top, end.top, progress)}px`
      fly.style.width = `${lerp(start.width, end.width, progress)}px`
      fly.style.height = `${lerp(start.height, end.height, progress)}px`
      if (introRef.current) {
        introRef.current.style.opacity = String(1 - smoothStep(0.02, 0.2, progress))
      }
      if (firstImageRef.current) {
        firstImageRef.current.style.opacity = String(1 - smoothStep(0.06, 0.28, progress))
      }
      if (lastImageRef.current) {
        lastImageRef.current.style.opacity = String(smoothStep(0.1, 0.32, progress))
      }
      if (detailRef.current) {
        detailRef.current.style.opacity = String(smoothStep(0.6, 0.9, progress))
      }
    }
    const onScroll = () => {
      if (!frame) {
        frame = requestAnimationFrame(update)
      }
    }
    update()
    window.addEventListener('scroll', onScroll, { passive: true })
    window.addEventListener('resize', onScroll)
    return () => {
      window.removeEventListener('scroll', onScroll)
      window.removeEventListener('resize', onScroll)
      if (frame) {
        cancelAnimationFrame(frame)
      }
    }
  }, [])

  if (!first || !last) {
    return null
  }
  return (
    <>
      <section className={styles.flyStart}>
        <div ref={introRef} className={styles.revealText}>
          <h2 className={styles.centeredTitle}>{first.heading}</h2>
          <div className={styles.centeredRule} aria-hidden="true" />
          <p className={styles.centeredSubhead}>{first.sub_heading}</p>
        </div>
        <div ref={slotARef} className={styles.flySlot} aria-hidden="true" />
      </section>

      <section className={styles.flyEnd}>
        <div className={styles.flyEndInner}>
          <div ref={detailRef} className={styles.flyDetail}>
            <h2 className={styles.featureTitle}>{last.heading}</h2>
            <div className={styles.featureRule} aria-hidden="true" />
            <p className={styles.featureSubhead}>{last.sub_heading}</p>
            <p className={styles.featureBody}>{last.description}</p>
            {trailingAction && (
              <Action action={trailingAction} onOpenVideo={onOpenVideo} />
            )}
          </div>
          <div ref={slotBRef} className={styles.flySlot} aria-hidden="true" />
        </div>
      </section>

      <div ref={flyRef} className={styles.flyImage} aria-hidden="true">
        <img
          ref={firstImageRef}
          src={resolveBlogAssetUrl(first.image?.fetch_url || first.image?.file_url)}
          alt=""
          className={`${styles.revealImage} ${styles.revealPine}`}
        />
        <img
          ref={lastImageRef}
          src={resolveBlogAssetUrl(last.image?.fetch_url || last.image?.file_url)}
          alt=""
          className={`${styles.revealImage} ${styles.revealReal}`}
        />
      </div>
    </>
  )
}

function PlayIcon() {
  return (
    <span className={styles.videoPlay} aria-hidden="true">
      <svg viewBox="0 0 24 24" width="26" height="26">
        <path d="M8 5v14l11-7z" fill="currentColor" />
      </svg>
    </span>
  )
}

function extractYouTubeId(url: string) {
  try {
    const parsed = new URL(url)
    if (parsed.hostname.includes('youtu.be')) {
      return parsed.pathname.split('/').filter(Boolean)[0] ?? ''
    }
    return (
      parsed.searchParams.get('v') ??
      parsed.pathname.match(/\/(?:embed|shorts)\/([^/?]+)/)?.[1] ??
      ''
    )
  } catch {
    return url.match(/(?:v=|youtu\.be\/|embed\/|shorts\/)([A-Za-z0-9_-]+)/)?.[1] ?? ''
  }
}

function thumbnailUrl(youtubeId: string) {
  return `https://i.ytimg.com/vi/${youtubeId}/hqdefault.jpg`
}

function embedUrl(youtubeId: string) {
  return `https://www.youtube-nocookie.com/embed/${youtubeId}?autoplay=1&rel=0&modestbranding=1`
}
