import { postJson } from './client'
import { normalizeVideoList } from './normalize'
import type { IsLikedResponse, MessageResponse, Video } from './types'

export function like(videoId: number) {
  return postJson<MessageResponse>('/like/like', { video_id: videoId }, { authRequired: true })
}

export function unlike(videoId: number) {
  return postJson<MessageResponse>('/like/unlike', { video_id: videoId }, { authRequired: true })
}

export function isLiked(videoId: number) {
  return postJson<IsLikedResponse>('/like/isLiked', { video_id: videoId }, { authRequired: true })
}

export async function listMyLikedVideos() {
  const videos = await postJson<Video[] | null>('/like/listMyLikedVideos', {}, { authRequired: true })
  return normalizeVideoList(videos)
}
