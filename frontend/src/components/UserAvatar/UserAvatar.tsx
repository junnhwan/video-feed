import styles from './UserAvatar.module.css'

export default function UserAvatar({ username, id, size = 34 }: { username: string; id: number; size?: number }) {
  const initial = username.charAt(0).toUpperCase()
  const color = `hsl(${(id * 47) % 360}, 55%, 55%)`

  return (
    <div className={styles.avatar} style={{ width: size, height: size, background: color, fontSize: size * 0.42 }}>
      {initial}
    </div>
  )
}
