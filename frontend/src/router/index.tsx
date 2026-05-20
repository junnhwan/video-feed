import { type ReactNode } from 'react'
import { createBrowserRouter, Navigate, type RouteObject } from 'react-router-dom'
import { useAuth } from '../stores/auth'
import AppShell from '../components/AppShell/AppShell'

function ProtectedRoute({ children }: { children: ReactNode }) {
  const auth = useAuth()
  if (!auth.isLoggedIn) {
    return <Navigate to="/account" replace />
  }
  return <>{children}</>
}

// Lazy-loaded pages
import FeedView from '../pages/FeedView/FeedView'
import AuthView from '../pages/AuthView/AuthView'
import ProfileView from '../pages/ProfileView/ProfileView'
import UploadView from '../pages/UploadView/UploadView'
import InboxView from '../pages/InboxView/InboxView'
import SettingsView from '../pages/SettingsView/SettingsView'
import ChangePasswordView from '../pages/ChangePasswordView/ChangePasswordView'

// Placeholder pages — will be replaced in later phases
function Placeholder({ name }: { name: string }) {
  return (
    <div style={{ padding: 40, textAlign: 'center', color: 'var(--text-secondary)' }}>
      <h2>{name}</h2>
      <p>即将实现</p>
    </div>
  )
}

const routes: RouteObject[] = [
  {
    path: '/',
    element: <AppShell />,
    children: [
      { index: true, element: <FeedView /> },
      { path: 'hot', element: <Placeholder name="热榜" /> },
      { path: 'following', element: <ProtectedRoute><Placeholder name="关注流" /></ProtectedRoute> },
      { path: 'upload', element: <ProtectedRoute><UploadView /></ProtectedRoute> },
      { path: 'u/:id', element: <ProfileView /> },
      { path: 'messages', element: <ProtectedRoute><InboxView /></ProtectedRoute> },
      { path: 'messages/:peerId', element: <ProtectedRoute><InboxView /></ProtectedRoute> },
      { path: 'account', element: <AuthView /> },
      { path: 'settings', element: <ProtectedRoute><SettingsView /></ProtectedRoute> },
      { path: 'change-password', element: <ProtectedRoute><ChangePasswordView /></ProtectedRoute> },
    ],
  },
]

export const router = createBrowserRouter(routes)
