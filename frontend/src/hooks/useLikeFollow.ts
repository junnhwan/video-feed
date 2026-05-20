import { useState, useCallback } from 'react'
import { ApiError } from '../api/client'
import * as likeApi from '../api/like'
import type { FeedVideoItem } from '../api/types'
import { useAuth } from '../stores/auth'
import { useSocial } from '../stores/social'
import { useToast } from '../stores/toast'

export function useLikeFollow(needLogin: () => void) {
  const auth = useAuth()
  const social = useSocial()
  const toast = useToast()

  const [likeBusy, setLikeBusy] = useState<Record<number, boolean>>({})
  const [followBusy, setFollowBusy] = useState<Record<number, boolean>>({})

  const toggleLike = useCallback(async (item: FeedVideoItem) => {
    if (!auth.isLoggedIn) return needLogin()
    if (likeBusy[item.id]) return
    setLikeBusy((prev) => ({ ...prev, [item.id]: true }))
    try {
      if (item.is_liked) await likeApi.unlike(item.id)
      else await likeApi.like(item.id)
      item.is_liked = !item.is_liked
      item.likes_count = Math.max(0, item.likes_count + (item.is_liked ? 1 : -1))
    } catch (e) {
      toast.error(e instanceof ApiError ? e.message : String(e))
    } finally {
      setLikeBusy((prev) => ({ ...prev, [item.id]: false }))
    }
  }, [auth.isLoggedIn, likeBusy, needLogin, toast])

  const toggleFollow = useCallback(async (authorId: number) => {
    if (!auth.isLoggedIn) return needLogin()
    if (followBusy[authorId]) return
    setFollowBusy((prev) => ({ ...prev, [authorId]: true }))
    try {
      if (social.isFollowing(authorId)) {
        await social.unfollow(authorId)
        toast.info('已取关')
      } else {
        await social.follow(authorId)
        toast.success('已关注')
      }
    } catch (e) {
      toast.error(e instanceof ApiError ? e.message : String(e))
    } finally {
      setFollowBusy((prev) => ({ ...prev, [authorId]: false }))
    }
  }, [auth.isLoggedIn, followBusy, needLogin, social, toast])

  const share = useCallback(async (item: FeedVideoItem) => {
    const url = `${location.origin}/u/${item.author.id}`
    try {
      await navigator.clipboard.writeText(url)
      toast.success('链接已复制')
    } catch {
      window.prompt('复制链接', url)
    }
  }, [toast])

  return { likeBusy, followBusy, toggleLike, toggleFollow, share }
}
