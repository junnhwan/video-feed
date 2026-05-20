import { useState, useEffect, useRef, useCallback } from 'react'
import * as notifApi from '../api/notification'
import type { Notification } from '../api/types'
import { useAuth } from '../stores/auth'
import { useToast } from '../stores/toast'

type NotificationState = {
  notifications: Notification[]
  unreadCount: number
  markAllRead: () => Promise<void>
  markSingleRead: (id: number) => Promise<void>
}

const BACKOFF_BASE = 1000
const BACKOFF_MAX = 30000

export function useNotifications(): NotificationState {
  const auth = useAuth()
  const toast = useToast()

  const [notifications, setNotifications] = useState<Notification[]>([])
  const [unreadCount, setUnreadCount] = useState(0)

  const esRef = useRef<EventSource | null>(null)
  const backoffRef = useRef(BACKOFF_BASE)
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const tokenRef = useRef<string | null>(null)

  const clearReconnect = useCallback(() => {
    if (timerRef.current !== null) {
      clearTimeout(timerRef.current)
      timerRef.current = null
    }
  }, [])

  const closeEs = useCallback(() => {
    if (esRef.current) {
      esRef.current.close()
      esRef.current = null
    }
    clearReconnect()
  }, [clearReconnect])

  const markAllRead = useCallback(async () => {
    try {
      await notifApi.markRead()
      setNotifications((prev) => prev.map((n) => ({ ...n, is_read: true })))
      setUnreadCount(0)
    } catch (e) {
      toast.error(String(e))
    }
  }, [toast])

  const markSingleRead = useCallback(async (id: number) => {
    try {
      await notifApi.markRead(id)
      setNotifications((prev) =>
        prev.map((n) => (n.id === id ? { ...n, is_read: true } : n)),
      )
      setUnreadCount((c) => Math.max(0, c - 1))
    } catch (e) {
      toast.error(String(e))
    }
  }, [toast])

  useEffect(() => {
    if (!auth.isLoggedIn || !auth.token) {
      closeEs()
      setNotifications([])
      setUnreadCount(0)
      tokenRef.current = null
      return
    }

    // Token changed -> reconnect
    if (tokenRef.current === auth.token) return
    tokenRef.current = auth.token
    closeEs()

    // Fetch initial unread count
    notifApi.unreadCount().then((res) => setUnreadCount(res.count)).catch(() => {})

    // Fetch initial notification list
    notifApi.listNotifications().then((res) => setNotifications(res.notifications)).catch(() => {})

    const connect = () => {
      const es = notifApi.streamNotifications(auth.token!)
      esRef.current = es

      es.onmessage = (e) => {
        try {
          const notif: Notification = JSON.parse(e.data)
          setNotifications((prev) => [notif, ...prev])
          setUnreadCount((c) => c + 1)
        } catch {
          // Ignore malformed payloads
        }
      }

      es.onerror = () => {
        es.close()
        esRef.current = null
        const delay = backoffRef.current
        backoffRef.current = Math.min(delay * 2, BACKOFF_MAX)
        timerRef.current = setTimeout(connect, delay)
      }

      // Reset backoff on successful open
      es.onopen = () => {
        backoffRef.current = BACKOFF_BASE
      }
    }

    connect()

    return () => {
      closeEs()
    }
  }, [auth.isLoggedIn, auth.token, closeEs])

  return { notifications, unreadCount, markAllRead, markSingleRead }
}
