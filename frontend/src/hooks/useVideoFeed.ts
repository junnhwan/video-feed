import { useState, useCallback, useRef } from 'react'
import { ApiError } from '../api/client'
import * as feedApi from '../api/feed'
import type { FeedVideoItem } from '../api/types'
import { useAuth } from '../stores/auth'

export type TabKey = 'recommend' | 'hot' | 'following'

type FeedState = {
  items: FeedVideoItem[]
  loading: boolean
  error: string
  hasMore: boolean
}

type RecommendState = FeedState & { nextTime: number }
type HotState = FeedState & { nextLikesCountBefore?: number; nextIdBefore?: number }
type FollowingState = FeedState & { nextTime: number }

export function useVideoFeed() {
  const auth = useAuth()
  const [tab, setTab] = useState<TabKey>('recommend')

  const [recommend, setRecommend] = useState<RecommendState>({
    items: [], loading: false, error: '', hasMore: false, nextTime: 0,
  })
  const [hot, setHot] = useState<HotState>({
    items: [], loading: false, error: '', hasMore: false,
  })
  const [following, setFollowing] = useState<FollowingState>({
    items: [], loading: false, error: '', hasMore: false, nextTime: 0,
  })

  const recommendRef = useRef(recommend)
  recommendRef.current = recommend
  const hotRef = useRef(hot)
  hotRef.current = hot
  const followingRef = useRef(following)
  followingRef.current = following

  const loadRecommend = useCallback(async (reset: boolean) => {
    const s = recommendRef.current
    if (s.loading) return
    setRecommend((prev) => ({ ...prev, loading: true, error: '' }))
    try {
      const res = await feedApi.listLatest({ limit: 10, latest_time: reset ? 0 : s.nextTime })
      setRecommend((prev) => ({
        items: reset ? res.video_list : prev.items.concat(res.video_list),
        loading: false, error: '', hasMore: res.has_more, nextTime: res.next_time,
      }))
    } catch (e) {
      setRecommend((prev) => ({ ...prev, loading: false, error: e instanceof ApiError ? e.message : String(e) }))
    }
  }, [])

  const loadHot = useCallback(async (reset: boolean) => {
    const s = hotRef.current
    if (s.loading) return
    setHot((prev) => ({ ...prev, loading: true, error: '' }))
    try {
      const res = await feedApi.listLikesCount({
        limit: 10,
        likes_count_before: reset ? undefined : s.nextLikesCountBefore,
        id_before: reset ? undefined : s.nextIdBefore,
      })
      setHot((prev) => ({
        items: reset ? res.video_list : prev.items.concat(res.video_list),
        loading: false, error: '', hasMore: res.has_more,
        nextLikesCountBefore: res.next_likes_count_before,
        nextIdBefore: res.next_id_before,
      }))
    } catch (e) {
      setHot((prev) => ({ ...prev, loading: false, error: e instanceof ApiError ? e.message : String(e) }))
    }
  }, [])

  const loadFollowing = useCallback(async (reset: boolean) => {
    if (!auth.isLoggedIn) {
      setFollowing((prev) => ({ ...prev, error: '登录后才能查看关注流' }))
      return
    }
    const s = followingRef.current
    if (s.loading) return
    setFollowing((prev) => ({ ...prev, loading: true, error: '' }))
    try {
      const res = await feedApi.listByFollowing({ limit: 10, latest_time: reset ? 0 : s.nextTime })
      setFollowing((prev) => ({
        items: reset ? res.video_list : prev.items.concat(res.video_list),
        loading: false, error: '', hasMore: res.has_more, nextTime: res.next_time,
      }))
    } catch (e) {
      setFollowing((prev) => ({ ...prev, loading: false, error: e instanceof ApiError ? e.message : String(e) }))
    }
  }, [auth.isLoggedIn])

  const getCurrentState = useCallback((): FeedState => {
    if (tab === 'hot') return hot
    if (tab === 'following') return following
    return recommend
  }, [tab, recommend, hot, following])

  const ensureTabLoaded = useCallback(async () => {
    if (tab === 'recommend' && recommend.items.length === 0) await loadRecommend(true)
    if (tab === 'hot' && hot.items.length === 0) await loadHot(true)
    if (tab === 'following' && following.items.length === 0) await loadFollowing(true)
  }, [tab, recommend.items.length, hot.items.length, following.items.length, loadRecommend, loadHot, loadFollowing])

  const loadMoreIfNeeded = useCallback(async (activeIndex: number) => {
    const items = getCurrentState().items
    if (items.length === 0 || activeIndex < items.length - 3) return
    if (tab === 'recommend' && recommend.hasMore) await loadRecommend(false)
    if (tab === 'hot' && hot.hasMore) await loadHot(false)
    if (tab === 'following' && following.hasMore) await loadFollowing(false)
  }, [tab, recommend.hasMore, hot.hasMore, following.hasMore, getCurrentState, loadRecommend, loadHot, loadFollowing])

  return {
    tab, setTab,
    recommend, hot, following,
    getCurrentState,
    loadRecommend, loadHot, loadFollowing,
    ensureTabLoaded, loadMoreIfNeeded,
  }
}
