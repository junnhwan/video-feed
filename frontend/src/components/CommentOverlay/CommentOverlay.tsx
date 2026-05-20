import { useState, useEffect, useCallback } from 'react'
import { ApiError } from '../../api/client'
import * as commentApi from '../../api/comment'
import type { Comment, FeedVideoItem } from '../../api/types'
import { useAuth } from '../../stores/auth'
import { useToast } from '../../stores/toast'
import styles from './CommentOverlay.module.css'

export default function CommentOverlay({
  video,
  onClose,
}: {
  video: FeedVideoItem
  onClose: () => void
}) {
  const auth = useAuth()
  const toast = useToast()

  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [comments, setComments] = useState<Comment[]>([])
  const [content, setContent] = useState('')

  const loadComments = useCallback(async () => {
    setLoading(true)
    setError('')
    try {
      const list = await commentApi.listAll(video.id)
      setComments(list)
    } catch (e) {
      setError(e instanceof ApiError ? e.message : String(e))
    } finally {
      setLoading(false)
    }
  }, [video.id])

  useEffect(() => { loadComments() }, [loadComments])

  async function publishComment() {
    if (!auth.isLoggedIn) { toast.error('请先登录'); return }
    const text = content.trim()
    if (!text) return
    setLoading(true)
    try {
      await commentApi.publish(video.id, text)
      setContent('')
      await loadComments()
      toast.success('评论已发布')
    } catch (e) {
      const msg = e instanceof ApiError ? e.message : String(e)
      setError(msg)
      toast.error(msg)
    } finally {
      setLoading(false)
    }
  }

  async function deleteComment(commentId: number) {
    if (!confirm('确认删除这条评论？')) return
    setLoading(true)
    try {
      await commentApi.remove(commentId)
      await loadComments()
      toast.info('评论已删除')
    } catch (e) {
      const msg = e instanceof ApiError ? e.message : String(e)
      setError(msg)
      toast.error(msg)
    } finally {
      setLoading(false)
    }
  }

  const myId = auth.claims?.account_id

  return (
    <div className={styles.backdrop} onClick={(e) => e.target === e.currentTarget && onClose()}>
      <div className={styles.drawer}>
        <div className={styles.head}>
          <div className={styles.title}>{video.title}</div>
          <button className={styles.closeBtn} onClick={onClose}>×</button>
        </div>

        <div className={styles.body}>
          {loading && comments.length === 0 && <div className={styles.hint}>加载中...</div>}
          {error && !comments.length && <div className={`${styles.hint} ${styles.bad}`}>{error}</div>}
          {!loading && !error && comments.length === 0 && <div className={styles.hint}>暂无评论</div>}

          {comments.map((c) => (
            <div key={c.id} className={styles.comment}>
              <div className={styles.commentTop}>
                <div className={styles.commentUser}>{c.username}</div>
                <div className={styles.commentMeta}>#{c.id} · {new Date(c.created_at).toLocaleString()}</div>
              </div>
              <div className={styles.commentContent}>{c.content}</div>
              {myId && myId === c.author_id && (
                <div className={styles.commentActions}>
                  <button className={`${styles.chip} ${styles.danger}`} disabled={loading} onClick={() => deleteComment(c.id)}>
                    删除
                  </button>
                </div>
              )}
            </div>
          ))}
        </div>

        <div className={styles.foot}>
          <textarea
            className={styles.textarea}
            value={content}
            onChange={(e) => setContent(e.target.value)}
            placeholder="说点什么..."
            disabled={loading}
          />
          <div className={styles.footActions}>
            <button className={styles.chip} disabled={loading} onClick={loadComments}>刷新</button>
            <button
              className={`${styles.chip} ${styles.primary}`}
              disabled={loading || !content.trim()}
              onClick={publishComment}
            >
              发送
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}
