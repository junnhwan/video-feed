import { useState, useRef, useCallback, type FormEvent } from 'react'
import { useNavigate } from 'react-router-dom'
import SparkMD5 from 'spark-md5'
import { useAuth } from '../../stores/auth'
import { useToast } from '../../stores/toast'
import * as videoApi from '../../api/video'
import { ApiError } from '../../api/client'
import styles from './UploadView.module.css'

/* ---------- constants ---------- */

const CHUNK_SIZE = 5 * 1024 * 1024          // 5 MB per chunk
const HASH_READ_SIZE = 2 * 1024 * 1024      // 2 MB per hash read
const LARGE_FILE_THRESHOLD = 10 * 1024 * 1024 // 10 MB — above this use chunked upload
const MAX_CONCURRENT = 3
const MAX_RETRIES = 3

/* ---------- helpers ---------- */

function computeFileMd5(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    const spark = new SparkMD5.ArrayBuffer()
    let offset = 0

    reader.onload = (e) => {
      if (e.target?.result) spark.append(e.target.result as ArrayBuffer)
      offset += HASH_READ_SIZE
      if (offset < file.size) {
        readNext()
      } else {
        resolve(spark.end())
      }
    }

    reader.onerror = () => reject(reader.error)

    function readNext() {
      const slice = file.slice(offset, Math.min(offset + HASH_READ_SIZE, file.size))
      reader.readAsArrayBuffer(slice)
    }

    readNext()
  })
}

function computeBlobMd5(blob: Blob): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = (e) => {
      const spark = new SparkMD5.ArrayBuffer()
      spark.append(e.target!.result as ArrayBuffer)
      resolve(spark.end())
    }
    reader.onerror = () => reject(reader.error)
    reader.readAsArrayBuffer(blob)
  })
}

/* ---------- stage type ---------- */

type Stage =
  | { phase: 'idle' }
  | { phase: 'hashing' }
  | { phase: 'uploading_chunks'; done: number; total: number }
  | { phase: 'uploading_video' }
  | { phase: 'uploading_cover' }
  | { phase: 'publishing' }

function stageLabel(stage: Stage): string {
  switch (stage.phase) {
    case 'idle':         return ''
    case 'hashing':      return '计算哈希'
    case 'uploading_chunks': return `上传视频中 (${stage.done}/${stage.total})`
    case 'uploading_video':  return '上传视频中'
    case 'uploading_cover':  return '上传封面'
    case 'publishing':       return '发布中'
  }
}

function stagePercent(stage: Stage): number {
  switch (stage.phase) {
    case 'idle':             return 0
    case 'hashing':          return 5
    case 'uploading_chunks': return 5 + Math.round((stage.done / stage.total) * 70)
    case 'uploading_video':  return 50
    case 'uploading_cover':  return 80
    case 'publishing':       return 95
  }
}

/* ---------- component ---------- */

export default function UploadView() {
  const [title, setTitle] = useState('')
  const [description, setDescription] = useState('')
  const [videoFile, setVideoFile] = useState<File | null>(null)
  const [coverFile, setCoverFile] = useState<File | null>(null)
  const [stage, setStage] = useState<Stage>({ phase: 'idle' })
  const [errorMsg, setErrorMsg] = useState('')

  const videoInputRef = useRef<HTMLInputElement>(null)
  const coverInputRef = useRef<HTMLInputElement>(null)

  const toast = useToast()
  const auth = useAuth()
  const navigate = useNavigate()

  const busy = stage.phase !== 'idle'

  /* ---- video picker ---- */
  const handleVideoChange = useCallback((e: React.ChangeEvent<HTMLInputElement>) => {
    const f = e.target.files?.[0]
    if (!f) return
    if (!f.name.toLowerCase().endsWith('.mp4') && f.type !== 'video/mp4') {
      toast.error('请选择 .mp4 格式的视频文件')
      return
    }
    setVideoFile(f)
  }, [toast])

  /* ---- cover picker ---- */
  const handleCoverChange = useCallback((e: React.ChangeEvent<HTMLInputElement>) => {
    const f = e.target.files?.[0]
    if (!f) return
    setCoverFile(f)
  }, [])

  /* ---- chunked upload ---- */
  async function doChunkedUpload(file: File) {
    // 1. compute hash
    setStage({ phase: 'hashing' })
    const fileHash = await computeFileMd5(file)

    const totalChunks = Math.ceil(file.size / CHUNK_SIZE)

    // 2. init
    const initRes = await videoApi.initChunkUpload({
      filename: file.name,
      file_size: file.size,
      chunk_size: CHUNK_SIZE,
      total_chunks: totalChunks,
      file_hash: fileHash,
    })

    const { upload_id, uploaded_chunks } = initRes

    // 3. upload pending chunks with concurrency
    const pendingIndices: number[] = []
    for (let i = 0; i < totalChunks; i++) {
      if (!uploaded_chunks.includes(i)) pendingIndices.push(i)
    }

    let doneCount = uploaded_chunks.length

    setStage({ phase: 'uploading_chunks', done: doneCount, total: totalChunks })

    const queue = [...pendingIndices]

    async function uploadOneChunk(index: number, attempt: number): Promise<void> {
      const start = index * CHUNK_SIZE
      const end = Math.min(start + CHUNK_SIZE, file.size)
      const blob = file.slice(start, end)
      const chunkHash = await computeBlobMd5(blob)

      try {
        await videoApi.uploadChunk(upload_id, index, chunkHash, blob)
        doneCount++
        setStage({ phase: 'uploading_chunks', done: doneCount, total: totalChunks })
      } catch (err) {
        if (attempt < MAX_RETRIES) {
          return uploadOneChunk(index, attempt + 1)
        }
        throw err
      }
    }

    // concurrent worker pool
    await Promise.all(
      Array.from({ length: Math.min(MAX_CONCURRENT, queue.length) }, async () => {
        while (queue.length > 0) {
          const idx = queue.shift()!
          await uploadOneChunk(idx, 0)
        }
      }),
    )

    // 4. complete
    const completeRes = await videoApi.completeChunkUpload(upload_id)
    return completeRes.play_url ?? completeRes.url
  }

  /* ---- simple upload (<10 MB) ---- */
  async function doSimpleUpload(file: File) {
    setStage({ phase: 'uploading_video' })
    const res = await videoApi.uploadVideo(file)
    return res.play_url ?? res.url
  }

  /* ---- submit ---- */
  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    if (!auth.isLoggedIn) {
      toast.error('请先登录')
      return
    }
    if (!videoFile) {
      toast.error('请选择视频文件')
      return
    }
    if (!coverFile) {
      toast.error('请选择封面图片')
      return
    }
    if (!title.trim()) {
      toast.error('请填写标题')
      return
    }

    setErrorMsg('')

    try {
      // upload video (chunked or simple)
      const isLarge = videoFile.size >= LARGE_FILE_THRESHOLD
      const playUrl = isLarge
        ? await doChunkedUpload(videoFile)
        : await doSimpleUpload(videoFile)

      // upload cover
      setStage({ phase: 'uploading_cover' })
      const coverRes = await videoApi.uploadCover(coverFile)
      const coverUrl = coverRes.cover_url ?? coverRes.url

      // publish
      setStage({ phase: 'publishing' })
      await videoApi.publishVideo({
        title: title.trim(),
        description: description.trim(),
        play_url: playUrl,
        cover_url: coverUrl,
      })

      toast.success('发布成功')
      navigate('/')
    } catch (err: unknown) {
      const msg = err instanceof ApiError ? err.message : (err instanceof Error ? err.message : '上传失败')
      setErrorMsg(msg)
      toast.error(msg)
    } finally {
      setStage({ phase: 'idle' })
    }
  }

  /* ---- preview URLs ---- */
  const videoPreviewUrl = videoFile ? URL.createObjectURL(videoFile) : null
  const coverPreviewUrl = coverFile ? URL.createObjectURL(coverFile) : null

  return (
    <div className={styles.page}>
      <h2 className={styles.heading}>发布视频</h2>

      <div className={styles.card}>
        <form className={styles.form} onSubmit={handleSubmit}>
          {/* title */}
          <label className={styles.label}>
            标题
            <input
              className={styles.input}
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              placeholder="给你的视频起个标题"
              maxLength={128}
              disabled={busy}
            />
          </label>

          {/* description */}
          <label className={styles.label}>
            描述
            <textarea
              className={styles.textarea}
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder="添加视频描述（可选）"
              maxLength={512}
              disabled={busy}
            />
          </label>

          {/* file pickers */}
          <div className={styles.fileRow}>
            {/* video picker */}
            <label className={styles.fileLabel}>
              {videoFile ? (
                <>
                  <video
                    className={styles.preview}
                    src={videoPreviewUrl!}
                    muted
                    playsInline
                  />
                  <span className={styles.previewName}>{videoFile.name}</span>
                </>
              ) : (
                <>
                  <span className={styles.fileIcon}>🎬</span>
                  <span className={styles.fileHint}>选择 .mp4 视频</span>
                </>
              )}
              <input
                ref={videoInputRef}
                className={styles.fileInput}
                type="file"
                accept="video/mp4,.mp4"
                onChange={handleVideoChange}
                disabled={busy}
              />
            </label>

            {/* cover picker */}
            <label className={styles.fileLabel}>
              {coverFile ? (
                <>
                  <img
                    className={styles.preview}
                    src={coverPreviewUrl!}
                    alt="封面预览"
                  />
                  <span className={styles.previewName}>{coverFile.name}</span>
                </>
              ) : (
                <>
                  <span className={styles.fileIcon}>🖼</span>
                  <span className={styles.fileHint}>选择封面图片</span>
                </>
              )}
              <input
                ref={coverInputRef}
                className={styles.fileInput}
                type="file"
                accept="image/*"
                onChange={handleCoverChange}
                disabled={busy}
              />
            </label>
          </div>

          {/* progress */}
          {stage.phase !== 'idle' && (
            <div className={styles.progressSection}>
              <div className={styles.progressLabel}>
                <span>{stageLabel(stage)}</span>
                <span>{stagePercent(stage)}%</span>
              </div>
              <div className={styles.progressTrack}>
                <div
                  className={styles.progressFill}
                  style={{ width: `${stagePercent(stage)}%` }}
                />
              </div>
            </div>
          )}

          {errorMsg && <p className={styles.errorText}>{errorMsg}</p>}

          {/* actions */}
          <div className={styles.btnRow}>
            <button
              type="button"
              className={styles.btn}
              onClick={() => navigate('/')}
              disabled={busy}
            >
              取消
            </button>
            <button
              type="submit"
              className={`${styles.btn} ${styles.primary}`}
              disabled={busy || !videoFile || !coverFile || !title.trim()}
            >
              {busy ? '发布中...' : '发布'}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}
