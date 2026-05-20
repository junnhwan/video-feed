import { useState, useRef, useEffect } from 'react'
import { useNotifications } from '../../hooks/useNotifications'
import NotificationList from '../NotificationList/NotificationList'
import styles from './NotificationBadge.module.css'

export default function NotificationBadge() {
  const { unreadCount } = useNotifications()
  const [open, setOpen] = useState(false)
  const wrapRef = useRef<HTMLDivElement>(null)

  // Close on outside click
  useEffect(() => {
    if (!open) return
    const handler = (e: MouseEvent) => {
      if (wrapRef.current && !wrapRef.current.contains(e.target as Node)) {
        setOpen(false)
      }
    }
    document.addEventListener('mousedown', handler)
    return () => document.removeEventListener('mousedown', handler)
  }, [open])

  return (
    <div className={styles.badgeWrap} ref={wrapRef}>
      <button className={styles.bell} onClick={() => setOpen((v) => !v)} aria-label="通知">
        🔔
        {unreadCount > 0 && (
          <span className={styles.count}>{unreadCount > 99 ? '99+' : unreadCount}</span>
        )}
      </button>

      {open && (
        <>
          <div className={styles.overlay} onClick={() => setOpen(false)} />
          <NotificationList onClose={() => setOpen(false)} />
        </>
      )}
    </div>
  )
}
