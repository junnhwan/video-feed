import { useState, useRef, type FormEvent } from 'react'
import { useNavigate, Link } from 'react-router-dom'
import { useAuth } from '../../stores/auth'
import { useToast } from '../../stores/toast'
import * as accountApi from '../../api/account'
import styles from './SettingsView.module.css'

export default function SettingsView() {
  const auth = useAuth()
  const toast = useToast()
  const navigate = useNavigate()

  const username = auth.claims?.username ?? ''
  const accountId = auth.claims?.account_id ?? 0
  const initial = username.charAt(0).toUpperCase()
  const avatarColor = `hsl(${(accountId * 47) % 360}, 55%, 55%)`

  // ─── Rename ───────────────────────────────────────────────
  const [newUsername, setNewUsername] = useState('')
  const [renameLoading, setRenameLoading] = useState(false)

  async function handleRename(e: FormEvent) {
    e.preventDefault()
    const trimmed = newUsername.trim()
    if (!trimmed) return
    setRenameLoading(true)
    try {
      const res = await accountApi.rename(trimmed)
      auth.setToken(res.token)
      setNewUsername('')
      toast.success('用户名已更新')
    } catch (err: unknown) {
      toast.error(err instanceof Error ? err.message : '重命名失败')
    } finally {
      setRenameLoading(false)
    }
  }

  // ─── Avatar upload ────────────────────────────────────────
  const fileRef = useRef<HTMLInputElement>(null)
  const [avatarLoading, setAvatarLoading] = useState(false)
  const [previewUrl, setPreviewUrl] = useState<string | null>(null)

  async function handleAvatarChange() {
    const file = fileRef.current?.files?.[0]
    if (!file) return

    setAvatarLoading(true)
    try {
      const { avatar_url } = await accountApi.uploadAvatar(file)
      await accountApi.updateProfile({ avatar_url })
      setPreviewUrl(avatar_url)
      toast.success('头像已更新')
    } catch (err: unknown) {
      toast.error(err instanceof Error ? err.message : '头像上传失败')
    } finally {
      setAvatarLoading(false)
    }
  }

  // ─── Bio ──────────────────────────────────────────────────
  const [bio, setBio] = useState('')
  const [bioLoading, setBioLoading] = useState(false)

  async function handleBioSave(e: FormEvent) {
    e.preventDefault()
    setBioLoading(true)
    try {
      await accountApi.updateProfile({ bio })
      toast.success('简介已保存')
    } catch (err: unknown) {
      toast.error(err instanceof Error ? err.message : '保存失败')
    } finally {
      setBioLoading(false)
    }
  }

  // ─── Logout ───────────────────────────────────────────────
  const [logoutLoading, setLogoutLoading] = useState(false)

  async function handleLogout() {
    setLogoutLoading(true)
    try {
      await accountApi.logout()
    } catch {
      // proceed even if API call fails
    }
    auth.clearTokens()
    navigate('/account')
  }

  return (
    <div className={styles.page}>
      <div className={styles.card}>
        <h2 className={styles.title}>设置</h2>

        {/* ─── Profile header ─────────────────────────────── */}
        <div className={styles.profileHeader}>
          <div className={styles.avatarWrap}>
            {previewUrl ? (
              <img className={styles.avatarImg} src={previewUrl} alt={username} />
            ) : (
              <div className={styles.avatarFallback} style={{ background: avatarColor }}>
                {initial}
              </div>
            )}
            <button
              className={styles.avatarBtn}
              onClick={() => fileRef.current?.click()}
              disabled={avatarLoading}
            >
              {avatarLoading ? '上传中...' : '更换头像'}
            </button>
            <input
              ref={fileRef}
              type="file"
              accept=".jpg,.jpeg,.png,.webp"
              className={styles.hiddenInput}
              onChange={handleAvatarChange}
            />
          </div>
          <span className={styles.currentUsername}>{username}</span>
        </div>

        {/* ─── Rename ─────────────────────────────────────── */}
        <form className={styles.section} onSubmit={handleRename}>
          <h3 className={styles.sectionTitle}>修改用户名</h3>
          <div className={styles.row}>
            <input
              className={styles.input}
              value={newUsername}
              onChange={(e) => setNewUsername(e.target.value)}
              placeholder="输入新用户名"
              autoComplete="off"
            />
            <button className={`${styles.btn} ${styles.primary}`} type="submit" disabled={renameLoading}>
              {renameLoading ? '...' : '确认'}
            </button>
          </div>
        </form>

        {/* ─── Bio ────────────────────────────────────────── */}
        <form className={styles.section} onSubmit={handleBioSave}>
          <h3 className={styles.sectionTitle}>个人简介</h3>
          <textarea
            className={styles.textarea}
            value={bio}
            onChange={(e) => setBio(e.target.value)}
            placeholder="写点什么介绍一下自己..."
            rows={3}
          />
          <button className={`${styles.btn} ${styles.primary}`} type="submit" disabled={bioLoading}>
            {bioLoading ? '保存中...' : '保存简介'}
          </button>
        </form>

        {/* ─── Change password link ───────────────────────── */}
        <Link className={styles.link} to="/change-password">
          修改密码
        </Link>

        {/* ─── Logout ─────────────────────────────────────── */}
        <button
          className={`${styles.btn} ${styles.danger}`}
          onClick={handleLogout}
          disabled={logoutLoading}
        >
          {logoutLoading ? '退出中...' : '退出登录'}
        </button>
      </div>
    </div>
  )
}
