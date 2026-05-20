import { useState, useEffect, useRef, useCallback, type KeyboardEvent, type FormEvent } from 'react'
import { useAuth } from '../../stores/auth'
import { useToast } from '../../stores/toast'
import * as messageApi from '../../api/message'
import type { DirectMessage } from '../../api/types'
import { ApiError } from '../../api/client'
import UserAvatar from '../UserAvatar/UserAvatar'
import styles from './ChatWindow.module.css'

function formatTime(iso: string): string {
  try {
    const d = new Date(iso)
    const now = new Date()
    const pad = (n: number) => String(n).padStart(2, '0')
    const time = `${pad(d.getHours())}:${pad(d.getMinutes())}`

    const isToday =
      d.getFullYear() === now.getFullYear() &&
      d.getMonth() === now.getMonth() &&
      d.getDate() === now.getDate()

    if (isToday) return time

    const yesterday = new Date(now)
    yesterday.setDate(yesterday.getDate() - 1)
    const isYesterday =
      d.getFullYear() === yesterday.getFullYear() &&
      d.getMonth() === yesterday.getMonth() &&
      d.getDate() === yesterday.getDate()

    if (isYesterday) return `昨天 ${time}`

    return `${pad(d.getMonth() + 1)}/${pad(d.getDate())} ${time}`
  } catch {
    return ''
  }
}

type Props = {
  peerId: number
  peerName: string
}

export default function ChatWindow({ peerId, peerName }: Props) {
  const auth = useAuth()
  const toast = useToast()

  const [messages, setMessages] = useState<DirectMessage[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [input, setInput] = useState('')
  const [sending, setSending] = useState(false)

  const messagesEndRef = useRef<HTMLDivElement>(null)
  const textareaRef = useRef<HTMLTextAreaElement>(null)

  const myId = auth.claims?.account_id ?? 0

  // Load messages on mount or when peer changes
  useEffect(() => {
    let cancelled = false
    setLoading(true)
    setError(null)
    setMessages([])

    messageApi
      .listMessages(peerId)
      .then((res) => {
        if (!cancelled) setMessages(res.messages)
      })
      .catch((err: unknown) => {
        if (!cancelled) {
          const msg = err instanceof ApiError ? err.message : '加载消息失败'
          setError(msg)
        }
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })

    return () => {
      cancelled = true
    }
  }, [peerId])

  // Auto-scroll to bottom
  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [messages, loading])

  // Auto-resize textarea
  useEffect(() => {
    const el = textareaRef.current
    if (!el) return
    el.style.height = 'auto'
    el.style.height = `${Math.min(el.scrollHeight, 120)}px`
  }, [input])

  const handleSend = useCallback(async () => {
    const text = input.trim()
    if (!text || sending) return

    setSending(true)
    try {
      const msg = await messageApi.sendMessage(peerId, text)
      setMessages((prev) => [...prev, msg])
      setInput('')
    } catch (err: unknown) {
      const msg = err instanceof ApiError ? err.message : '发送失败'
      toast.error(msg)
    } finally {
      setSending(false)
    }
  }, [input, sending, peerId, toast])

  function handleKeyDown(e: KeyboardEvent<HTMLTextAreaElement>) {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault()
      handleSend()
    }
  }

  function handleSubmit(e: FormEvent) {
    e.preventDefault()
    handleSend()
  }

  return (
    <div className={styles.wrapper}>
      <div className={styles.header}>
        <UserAvatar username={peerName} id={peerId} size={32} />
        <span className={styles.headerName}>{peerName}</span>
      </div>

      {error && <div className={styles.errorBanner}>{error}</div>}

      {loading ? (
        <div className={styles.loading}>加载中...</div>
      ) : (
        <div className={styles.messages}>
          {messages.length === 0 && (
            <div className={styles.messagesEmpty}>暂无消息，发送第一条吧</div>
          )}
          {messages.map((msg) => {
            const isSent = msg.from_id === myId
            return (
              <div
                key={msg.id}
                className={`${styles.row} ${isSent ? styles.rowSent : styles.rowReceived}`}
              >
                <div className={`${styles.bubble} ${isSent ? styles.bubbleSent : styles.bubbleReceived}`}>
                  <div className={styles.bubbleText}>{msg.content}</div>
                  <div className={styles.bubbleTime}>{formatTime(msg.created_at)}</div>
                </div>
              </div>
            )
          })}
          <div ref={messagesEndRef} />
        </div>
      )}

      <form className={styles.inputBar} onSubmit={handleSubmit}>
        <textarea
          ref={textareaRef}
          className={styles.textarea}
          value={input}
          onChange={(e) => setInput(e.target.value)}
          onKeyDown={handleKeyDown}
          placeholder="输入消息..."
          rows={1}
        />
        <button className={styles.sendBtn} type="submit" disabled={sending || !input.trim()}>
          发送
        </button>
      </form>
    </div>
  )
}
