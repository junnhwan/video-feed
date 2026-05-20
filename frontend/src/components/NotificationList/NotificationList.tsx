import { useNotifications } from '../../hooks/useNotifications'
import type { Notification } from '../../api/types'
import styles from './NotificationList.module.css'

type Props = {
  onClose: () => void
}

const TYPE_ICON: Record<string, { emoji: string; cls: string }> = {
  like: { emoji: '♥', cls: styles.iconLike },
  comment: { emoji: '💬', cls: styles.iconComment },
  follow: { emoji: '👤', cls: styles.iconFollow },
}

function iconFor(type: string) {
  return TYPE_ICON[type] ?? { emoji: '•', cls: styles.iconLike }
}

function relativeTime(iso: string): string {
  const diff = Date.now() - new Date(iso).getTime()
  const sec = Math.floor(diff / 1000)
  if (sec < 60) return '刚刚'
  const min = Math.floor(sec / 60)
  if (min < 60) return `${min}分钟前`
  const hr = Math.floor(min / 60)
  if (hr < 24) return `${hr}小时前`
  const day = Math.floor(hr / 24)
  if (day < 30) return `${day}天前`
  return new Date(iso).toLocaleDateString()
}

export default function NotificationList({ onClose: _onClose }: Props) {
  const { notifications, markAllRead, markSingleRead } = useNotifications()

  const handleItemClick = (n: Notification) => {
    if (!n.is_read) markSingleRead(n.id)
    // Future: navigate to target (video, profile, etc.)
  }

  return (
    <div className={styles.panel}>
      <div className={styles.head}>
        <span className={styles.title}>通知</span>
        <button className={styles.markAll} onClick={markAllRead}>
          全部已读
        </button>
      </div>

      <div className={styles.body}>
        {notifications.length === 0 && (
          <div className={styles.empty}>暂无通知</div>
        )}

        {notifications.map((n) => {
          const icon = iconFor(n.type)
          return (
            <div
              key={n.id}
              className={`${styles.item} ${!n.is_read ? styles.itemUnread : ''}`}
              onClick={() => handleItemClick(n)}
            >
              <div className={`${styles.icon} ${icon.cls}`}>{icon.emoji}</div>
              <div className={styles.content}>
                <div className={styles.text}>{n.content}</div>
                <div className={styles.time}>{relativeTime(n.created_at)}</div>
              </div>
              {!n.is_read && <div className={styles.dot} />}
            </div>
          )
        })}
      </div>
    </div>
  )
}
