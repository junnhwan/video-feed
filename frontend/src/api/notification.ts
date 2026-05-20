import { postJson } from './client'
import type { ListNotificationsResponse, MessageResponse, UnreadCountResponse } from './types'

export function streamNotifications(token: string): EventSource {
  return new EventSource(`/api/notification/stream?token=${encodeURIComponent(token)}`)
}

export function listNotifications() {
  return postJson<ListNotificationsResponse>('/notification/list', {}, { authRequired: true })
}

export function markRead(id?: number) {
  return postJson<MessageResponse>('/notification/markRead', id ? { id } : {}, { authRequired: true })
}

export function unreadCount() {
  return postJson<UnreadCountResponse>('/notification/unreadCount', {}, { authRequired: true })
}
