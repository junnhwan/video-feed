import { useState, useRef, useCallback } from 'react'

export function useVideoPlayer() {
  const [muted, setMuted] = useState(true)
  const [activeIndex, setActiveIndex] = useState(0)
  const videoMap = useRef(new Map<number, HTMLVideoElement>())

  const setVideoRef = useCallback((id: number, el: HTMLVideoElement | null) => {
    if (el) {
      el.muted = muted
      videoMap.current.set(id, el)
    } else {
      videoMap.current.delete(id)
    }
  }, [muted])

  const scrollToIndex = useCallback((idx: number, totalItems: number, scroller: HTMLDivElement | null) => {
    if (!scroller) return
    const h = scroller.clientHeight
    if (!h) return
    const next = Math.max(0, Math.min(idx, Math.max(0, totalItems - 1)))
    scroller.scrollTo({ top: next * h, behavior: 'smooth' })
  }, [])

  const playActive = useCallback(async (activeItemId: number | undefined) => {
    if (!activeItemId) return
    for (const [id, v] of videoMap.current.entries()) {
      if (id === activeItemId) continue
      v.pause()
    }
    const video = videoMap.current.get(activeItemId)
    if (!video) return
    video.muted = muted
    try {
      await video.play()
    } catch {
      /* ignore autoplay errors */
    }
  }, [muted])

  const toggleMute = useCallback(() => {
    setMuted((prev) => {
      const next = !prev
      for (const v of videoMap.current.values()) v.muted = next
      return next
    })
  }, [])

  const togglePlayPause = useCallback((activeItemId: number | undefined) => {
    if (!activeItemId) return
    const video = videoMap.current.get(activeItemId)
    if (!video) return
    if (video.paused) void video.play()
    else video.pause()
  }, [])

  const clearVideos = useCallback(() => {
    videoMap.current.clear()
  }, [])

  return {
    muted, activeIndex, setActiveIndex,
    setVideoRef, scrollToIndex, playActive,
    toggleMute, togglePlayPause, clearVideos,
  }
}
