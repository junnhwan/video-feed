import { useState, useEffect, useCallback, useRef, useMemo } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { useAuth } from '../../stores/auth'
import { useSocial } from '../../stores/social'
import { useToast } from '../../stores/toast'
import { useVideoFeed } from '../../hooks/useVideoFeed'
import { useVideoPlayer } from '../../hooks/useVideoPlayer'
import { useLikeFollow } from '../../hooks/useLikeFollow'
import type { FeedVideoItem } from '../../api/types'
import UserAvatar from '../../components/UserAvatar/UserAvatar'
import CommentOverlay from '../../components/CommentOverlay/CommentOverlay'
import styles from './FeedView.module.css'

export default function FeedView() {
  const auth = useAuth()
  const social = useSocial()
  const toast = useToast()
  const navigate = useNavigate()

  const {
    tab, setTab,
    following: followingState,
    getCurrentState,
    ensureTabLoaded, loadMoreIfNeeded, loadFollowing,
  } = useVideoFeed()

  const player = useVideoPlayer()
  const needLogin = useCallback(() => { toast.error('请先登录'); navigate('/account') }, [toast, navigate])
  const { likeBusy, followBusy, toggleLike, toggleFollow, share } = useLikeFollow(needLogin)

  const scrollerRef = useRef<HTMLDivElement>(null)
  const [commentVideo, setCommentVideo] = useState<FeedVideoItem | null>(null)
  const myAccountId = auth.claims?.account_id ?? 0

  const currentState = getCurrentState()
  const items = currentState.items

  const activeItem = items[player.activeIndex] ?? null
  const visibleRange = useMemo(() => {
    const idx = player.activeIndex
    const len = items.length
    return { start: Math.max(0, idx - 1), end: Math.min(len - 1, idx + 1) }
  }, [player.activeIndex, items.length])

  // Play active video when index changes
  useEffect(() => {
    player.playActive(activeItem?.id)
    if (activeItem) loadMoreIfNeeded(player.activeIndex)
  }, [player.activeIndex])

  // Initial load
  useEffect(() => {
    ensureTabLoaded().then(() => {
      setTimeout(() => player.playActive(items[0]?.id), 100)
    })
  }, [])

  // Tab switch
  useEffect(() => {
    player.setActiveIndex(0)
    player.clearVideos()
    if (scrollerRef.current) scrollerRef.current.scrollTop = 0
    ensureTabLoaded().then(() => {
      setTimeout(() => player.playActive(getCurrentState().items[0]?.id), 100)
    })
  }, [tab])

  // Refresh following when auth changes
  useEffect(() => {
    if (tab === 'following' && auth.isLoggedIn && followingState.items.length === 0) {
      loadFollowing(true)
    }
  }, [auth.isLoggedIn, tab])

  // Scroll handler
  const onScroll = useCallback(() => {
    const el = scrollerRef.current
    if (!el) return
    const h = el.clientHeight
    if (!h) return
    const idx = Math.round(el.scrollTop / h)
    if (idx !== player.activeIndex) player.setActiveIndex(idx)
  }, [player.activeIndex])

  // Double-tap detection
  const lastTapRef = useRef(0)
  function handleStageClick(_e: React.MouseEvent, item: FeedVideoItem) {
    const now = Date.now()
    if (now - lastTapRef.current < 300) {
      toggleLike(item)
      lastTapRef.current = 0
    } else {
      lastTapRef.current = now
      player.togglePlayPause(activeItem?.id)
    }
  }

  // Keyboard shortcuts
  useEffect(() => {
    function onKeydown(e: KeyboardEvent) {
      const t = e.target as HTMLElement | null
      if (t && (t.tagName === 'INPUT' || t.tagName === 'TEXTAREA')) return
      if (commentVideo) return
      if (e.key === 'ArrowDown') {
        e.preventDefault()
        player.scrollToIndex(player.activeIndex + 1, items.length, scrollerRef.current)
      } else if (e.key === 'ArrowUp') {
        e.preventDefault()
        player.scrollToIndex(player.activeIndex - 1, items.length, scrollerRef.current)
      } else if (e.key === ' ') {
        e.preventDefault()
        player.togglePlayPause(activeItem?.id)
      } else if (e.key.toLowerCase() === 'm') {
        e.preventDefault()
        player.toggleMute()
        toast.info(player.muted ? '已静音' : '已取消静音')
      } else if (e.key.toLowerCase() === 'c') {
        if (activeItem) { e.preventDefault(); setCommentVideo(activeItem) }
      }
    }
    window.addEventListener('keydown', onKeydown)
    return () => window.removeEventListener('keydown', onKeydown)
  }, [player.activeIndex, items.length, activeItem, commentVideo])

  return (
    <div className={styles.page}>
      <div className={styles.tabs}>
        <button className={`${styles.tab} ${tab === 'recommend' ? styles.on : ''}`} onClick={() => setTab('recommend')}>推荐</button>
        <button className={`${styles.tab} ${tab === 'following' ? styles.on : ''}`} onClick={() => setTab('following')}>关注</button>
        <button className={`${styles.tab} ${tab === 'hot' ? styles.on : ''}`} onClick={() => setTab('hot')}>点赞榜</button>
        <div className={styles.tabsRight}>
          <button className={styles.chip} onClick={() => { player.toggleMute(); toast.info(player.muted ? '已静音' : '已取消静音') }}>
            {player.muted ? '静音' : '有声'}
          </button>
        </div>
      </div>

      <div ref={scrollerRef} className={styles.scroller} onScroll={onScroll}>
        {currentState.loading && items.length === 0 && <div className={styles.centerHint}>加载中...</div>}
        {currentState.error && items.length === 0 && <div className={`${styles.centerHint} ${styles.bad}`}>{currentState.error}</div>}
        {!currentState.loading && items.length === 0 && <div className={styles.centerHint}>没有内容</div>}

        {items.map((item, idx) => (
          <section
            key={`${tab}-${item.id}`}
            className={`${styles.slide} ${idx === player.activeIndex ? styles.active : ''}`}
            style={{ display: idx >= visibleRange.start && idx <= visibleRange.end ? undefined : 'none' }}
          >
            <div className={styles.stage} onClick={(e) => handleStageClick(e, item)}>
              <video
                className={styles.video}
                ref={(el) => player.setVideoRef(item.id, el)}
                src={item.play_url}
                poster={item.cover_url}
                playsInline
                preload="metadata"
                loop
              />
              <div className={styles.grad} />
              <div className={styles.meta}>
                <Link className={styles.authorLink} to={`/u/${item.author.id}`} onClick={(e) => e.stopPropagation()}>
                  <UserAvatar username={item.author.username} id={item.author.id} size={34} />
                  <span className={styles.authorName}>@{item.author.username}</span>
                </Link>
                <div className={styles.itemTitle}>{item.title}</div>
                {item.description && <div className={styles.desc}>{item.description}</div>}
              </div>
              <div className={styles.actions} onClick={(e) => e.stopPropagation()}>
                <button className={styles.act} disabled={!!likeBusy[item.id]} onClick={() => toggleLike(item)}>
                  <span className={`${styles.icon} ${item.is_liked ? styles.liked : ''}`}>♥</span>
                  <span className={styles.count}>{item.likes_count}</span>
                </button>
                <button className={styles.act} onClick={() => setCommentVideo(item)}>
                  <span className={styles.icon}>💬</span>
                  <span className={styles.count}>评论</span>
                </button>
                {(!myAccountId || myAccountId !== item.author.id) && (
                  <button className={styles.act} disabled={!!followBusy[item.author.id]} onClick={() => toggleFollow(item.author.id)}>
                    <span className={styles.icon}>＋</span>
                    <span className={styles.count}>{social.isFollowing(item.author.id) ? '已关注' : '关注'}</span>
                  </button>
                )}
                <button className={styles.act} onClick={() => share(item)}>
                  <span className={styles.icon}>↗</span>
                  <span className={styles.count}>分享</span>
                </button>
              </div>
              <div className={styles.hint}>
                <span className={styles.chip}>↑ ↓ 切换</span>
                <span className={styles.chip}>空格 暂停</span>
                <span className={styles.chip}>M 静音</span>
                <span className={styles.chip}>C 评论</span>
              </div>
            </div>
          </section>
        ))}
      </div>

      {commentVideo && <CommentOverlay video={commentVideo} onClose={() => setCommentVideo(null)} />}
    </div>
  )
}
