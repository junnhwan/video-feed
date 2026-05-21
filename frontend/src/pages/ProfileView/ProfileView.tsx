import { useState, useEffect, useCallback } from 'react'
import { useParams, useNavigate, Link } from 'react-router-dom'
import { useAuth } from '../../stores/auth'
import { useSocial } from '../../stores/social'
import { useToast } from '../../stores/toast'
import * as accountApi from '../../api/account'
import * as videoApi from '../../api/video'
import * as likeApi from '../../api/like'
import * as socialApi from '../../api/social'
import { ApiError } from '../../api/client'
import type { Account, Video, ProfileResponse } from '../../api/types'
import UserAvatar from '../../components/UserAvatar/UserAvatar'
import styles from './ProfileView.module.css'

type TabKey = 'works' | 'likes'
type ModalKey = 'none' | 'followers' | 'following'

/* ── Skeleton Component ── */
function ProfileSkeleton() {
  return (
    <div className={styles.skeleton}>
      <div className={styles.skHeader}>
        <div className={styles.skAvatar} />
        <div className={styles.skInfo}>
          <div className={`${styles.skLine} ${styles.skLineName}`} />
          <div className={`${styles.skLine} ${styles.skLineBio}`} />
          <div className={`${styles.skLine} ${styles.skLineStats}`} />
        </div>
      </div>
      <div className={styles.skGrid}>
        {Array.from({ length: 6 }).map((_, i) => (
          <div key={i} className={styles.skGridItem} />
        ))}
      </div>
    </div>
  )
}

export default function ProfileView() {
  const { id: routeId } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const auth = useAuth()
  const social = useSocial()
  const toast = useToast()

  const isSelf = !routeId || (auth.claims?.account_id != null && String(auth.claims.account_id) === routeId)
  const accountId = isSelf ? auth.claims!.account_id! : Number(routeId)

  const [profile, setProfile] = useState<ProfileResponse | null>(null)
  const [loadingProfile, setLoadingProfile] = useState(true)

  const [activeTab, setActiveTab] = useState<TabKey>('works')
  const [works, setWorks] = useState<Video[]>([])
  const [likedVideos, setLikedVideos] = useState<Video[]>([])
  const [loadingVideos, setLoadingVideos] = useState(false)

  const [followBusy, setFollowBusy] = useState(false)

  const [modal, setModal] = useState<ModalKey>('none')
  const [modalUsers, setModalUsers] = useState<Account[]>([])
  const [modalLoading, setModalLoading] = useState(false)

  useEffect(() => {
    let cancelled = false
    setLoadingProfile(true)
    accountApi
      .getProfile(accountId)
      .then((res) => {
        if (!cancelled) setProfile(res)
      })
      .catch((e) => {
        if (!cancelled) toast.error(e instanceof ApiError ? e.message : '加载失败')
      })
      .finally(() => {
        if (!cancelled) setLoadingProfile(false)
      })
    return () => { cancelled = true }
  }, [accountId, toast])

  useEffect(() => {
    let cancelled = false

    if (activeTab === 'works') {
      setLoadingVideos(true)
      videoApi
        .listByAuthorId(accountId)
        .then((list) => { if (!cancelled) setWorks(list) })
        .catch((e) => { if (!cancelled) toast.error(e instanceof ApiError ? e.message : '加载视频失败') })
        .finally(() => { if (!cancelled) setLoadingVideos(false) })
    } else if (activeTab === 'likes') {
      if (!isSelf || !auth.isLoggedIn) {
        setLikedVideos([])
        return
      }
      setLoadingVideos(true)
      likeApi
        .listMyLikedVideos()
        .then((list) => { if (!cancelled) setLikedVideos(list) })
        .catch((e) => { if (!cancelled) toast.error(e instanceof ApiError ? e.message : '加载失败') })
        .finally(() => { if (!cancelled) setLoadingVideos(false) })
    }

    return () => { cancelled = true }
  }, [activeTab, accountId, isSelf, auth.isLoggedIn, toast])

  const handleToggleFollow = useCallback(async () => {
    if (!auth.isLoggedIn) {
      navigate('/account')
      return
    }
    if (followBusy) return
    setFollowBusy(true)
    try {
      if (social.isFollowing(accountId)) {
        await social.unfollow(accountId)
        toast.info('已取关')
      } else {
        await social.follow(accountId)
        toast.success('已关注')
      }
    } catch (e) {
      toast.error(e instanceof ApiError ? e.message : String(e))
    } finally {
      setFollowBusy(false)
    }
  }, [auth.isLoggedIn, followBusy, accountId, social, toast, navigate])

  const openModal = useCallback(
    async (key: 'followers' | 'following') => {
      setModal(key)
      setModalLoading(true)
      setModalUsers([])
      try {
        if (key === 'followers') {
          const res = await socialApi.getAllFollowers(accountId)
          setModalUsers(res.followers)
        } else {
          const res = await socialApi.getAllVloggers(accountId)
          setModalUsers(res.vloggers)
        }
      } catch (e) {
        toast.error(e instanceof ApiError ? e.message : '加载列表失败')
      } finally {
        setModalLoading(false)
      }
    },
    [accountId, toast],
  )

  if (loadingProfile) {
    return (
      <div className={styles.page}>
        <ProfileSkeleton />
      </div>
    )
  }

  if (!profile) {
    return (
      <div className={styles.page}>
        <div className={styles.empty}>
          用户不存在
        </div>
      </div>
    )
  }

  const { account, total_likes, follower_count, vlogger_count } = profile
  const following = auth.isLoggedIn ? social.isFollowing(accountId) : false
  const videos = activeTab === 'works' ? works : likedVideos

  return (
    <div className={styles.page}>
      {/* ─── Header ──────────────────────────────────────── */}
      <div className={styles.header}>
        <div className={styles.avatarWrap}>
          <UserAvatar username={account.username} id={account.id} size={80} />
        </div>

        <div className={styles.info}>
          <h1 className={styles.username}>{account.username}</h1>
          {account.bio && <p className={styles.bio}>{account.bio}</p>}

          <div className={styles.stats}>
            <button className={styles.statBtn} onClick={() => openModal('following')}>
              <span className={styles.statNum}>{vlogger_count}</span>
              <span className={styles.statLabel}>关注</span>
            </button>
            <button className={styles.statBtn} onClick={() => openModal('followers')}>
              <span className={styles.statNum}>{follower_count}</span>
              <span className={styles.statLabel}>粉丝</span>
            </button>
            <div className={styles.statItem}>
              <span className={styles.statNum}>{total_likes}</span>
              <span className={styles.statLabel}>获赞</span>
            </div>
          </div>

          {!isSelf && (
            <button
              className={`${styles.followBtn} ${following ? styles.following : styles.notFollowing}`}
              onClick={handleToggleFollow}
              disabled={followBusy}
            >
              {followBusy ? '...' : following ? '已关注' : '关注'}
            </button>
          )}
        </div>
      </div>

      {/* ─── Tabs ────────────────────────────────────────── */}
      <div className={styles.tabs}>
        <button
          className={`${styles.tab} ${activeTab === 'works' ? styles.tabActive : ''}`}
          onClick={() => setActiveTab('works')}
        >
          作品
        </button>
        {isSelf && (
          <button
            className={`${styles.tab} ${activeTab === 'likes' ? styles.tabActive : ''}`}
            onClick={() => setActiveTab('likes')}
          >
            喜欢
          </button>
        )}
      </div>

      {/* ─── Video Grid ──────────────────────────────────── */}
      {loadingVideos ? (
        <div className={styles.gridLoading}>加载中...</div>
      ) : videos.length === 0 ? (
        <div className={styles.empty}>
          {activeTab === 'works' ? '还没有发布作品' : '还没有点赞过视频'}
        </div>
      ) : (
        <div className={styles.grid}>
          {videos.map((v) => (
            <Link key={v.id} className={styles.gridItem} to={`/?video=${v.id}`}>
              <img
                className={styles.cover}
                src={v.cover_url}
                alt={v.title}
                loading="lazy"
              />
              <div className={styles.playOverlay}>
                <div className={styles.playBtn}>
                  <svg viewBox="0 0 24 24"><path d="M8 5v14l11-7z"/></svg>
                </div>
              </div>
              <div className={styles.coverOverlay}>
                <span className={styles.coverTitle}>{v.title}</span>
                <span className={styles.coverLikes}>&#9829; {v.likes_count}</span>
              </div>
            </Link>
          ))}
        </div>
      )}

      {/* ─── Modal ───────────────────────────────────────── */}
      {modal !== 'none' && (
        <div className={styles.modalBackdrop} onClick={() => setModal('none')}>
          <div className={styles.modal} onClick={(e) => e.stopPropagation()}>
            <div className={styles.modalHeader}>
              <h2 className={styles.modalTitle}>{modal === 'followers' ? '粉丝' : '关注'}</h2>
              <button className={styles.modalClose} onClick={() => setModal('none')}>
                &times;
              </button>
            </div>
            <div className={styles.modalBody}>
              {modalLoading ? (
                <div className={styles.modalLoading}>加载中...</div>
              ) : modalUsers.length === 0 ? (
                <div className={styles.modalEmpty}>暂无用户</div>
              ) : (
                modalUsers.map((u) => (
                  <Link
                    key={u.id}
                    className={styles.modalUserRow}
                    to={`/u/${u.id}`}
                    onClick={() => setModal('none')}
                  >
                    <UserAvatar username={u.username} id={u.id} size={36} />
                    <span className={styles.modalUsername}>{u.username}</span>
                  </Link>
                ))
              )}
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
