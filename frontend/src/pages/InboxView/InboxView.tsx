import { useState, useEffect, useMemo, useCallback } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { useAuth } from '../../stores/auth'
import { useSocial } from '../../stores/social'
import UserAvatar from '../../components/UserAvatar/UserAvatar'
import ChatWindow from '../../components/ChatWindow/ChatWindow'
import styles from './InboxView.module.css'

const MOBILE_BREAKPOINT = 760

export default function InboxView() {
  const { peerId: peerIdStr } = useParams<{ peerId?: string }>()
  const navigate = useNavigate()
  const auth = useAuth()
  const social = useSocial()

  const myId = auth.claims?.account_id ?? 0
  const peerId = peerIdStr ? Number(peerIdStr) : null

  const [isMobile, setIsMobile] = useState(() => window.innerWidth <= MOBILE_BREAKPOINT)

  useEffect(() => {
    function onResize() {
      setIsMobile(window.innerWidth <= MOBILE_BREAKPOINT)
    }
    window.addEventListener('resize', onResize)
    return () => window.removeEventListener('resize', onResize)
  }, [])

  const contacts = useMemo(() => {
    const seen = new Set<number>()
    const result: { id: number; username: string }[] = []

    for (const u of social.followers) {
      if (u.id !== myId && !seen.has(u.id)) {
        seen.add(u.id)
        result.push({ id: u.id, username: u.username })
      }
    }
    for (const u of social.vloggers) {
      if (u.id !== myId && !seen.has(u.id)) {
        seen.add(u.id)
        result.push({ id: u.id, username: u.username })
      }
    }

    return result
  }, [social.followers, social.vloggers, myId])

  const activePeer = useMemo(
    () => (peerId !== null ? contacts.find((c) => c.id === peerId) : null),
    [contacts, peerId],
  )

  const hasChat = peerId !== null && !!activePeer

  const handleSelect = useCallback(
    (id: number) => {
      navigate(`/messages/${id}`)
    },
    [navigate],
  )

  const handleMobileBack = useCallback(() => {
    navigate('/messages')
  }, [navigate])

  const hideSidebar = isMobile && hasChat
  const hideChatArea = isMobile && !hasChat

  return (
    <div className={styles.page}>
      <div className={`${styles.sidebar} ${hideSidebar ? styles.sidebarHidden : ''}`}>
        <div className={styles.sidebarHeader}>私信</div>
        <div className={styles.contactList}>
          {contacts.length === 0 && (
            <div className={styles.contactsEmpty}>暂无联系人</div>
          )}
          {contacts.map((c) => (
            <div
              key={c.id}
              className={`${styles.contact} ${peerId === c.id ? styles.contactActive : ''}`}
              onClick={() => handleSelect(c.id)}
            >
              <UserAvatar username={c.username} id={c.id} size={36} />
              <span className={styles.contactName}>{c.username}</span>
            </div>
          ))}
        </div>
      </div>

      <div className={`${styles.chatArea} ${hideChatArea ? styles.chatHidden : ''}`}>
        {hasChat && activePeer ? (
          <>
            <div className={styles.mobileBack} onClick={handleMobileBack}>
              <span className={styles.backArrow}>&larr;</span>
              <span>返回</span>
            </div>
            <ChatWindow peerId={activePeer.id} peerName={activePeer.username} />
          </>
        ) : (
          <div className={styles.chatEmpty}>
            <span className={styles.chatEmptyIcon}>💬</span>
            <span className={styles.chatEmptyText}>选择联系人开始聊天</span>
          </div>
        )}
      </div>
    </div>
  )
}
