import { useEffect, useState } from 'react'

type InfoResponse = { name: string }
type HealthResponse = { status: string }
type User = { sub: string; username: string; name: string; email: string }
type LogoutResponse = { logout_url: string }

type State =
  | { kind: 'loading' }
  | { kind: 'error' }
  | { kind: 'ready'; name: string; status: string; user: User | null }

const AUTH_ERROR_MESSAGES: Record<string, string> = {
  invalid_state: '登入流程已逾時或無效，請重新登入。',
  access_denied: 'authentik 拒絕了這次登入。',
  token_error: '無法驗證登入結果，請重新登入。',
}

async function getJSON<T>(path: string, signal: AbortSignal): Promise<T> {
  const res = await fetch(path, { signal })
  if (!res.ok) {
    throw new Error(`${path} responded with ${res.status}`)
  }
  return (await res.json()) as T
}

// getMe returns the signed-in user, or null when the backend answers 401.
async function getMe(signal: AbortSignal): Promise<User | null> {
  const res = await fetch('/api/me', { signal })
  if (res.status === 401) return null
  if (!res.ok) {
    throw new Error(`/api/me responded with ${res.status}`)
  }
  return (await res.json()) as User
}

// takeAuthError reads ?auth_error= set by the login callback and removes it
// from the address bar, so a reload does not show the message again.
function takeAuthError(): string | null {
  const url = new URL(window.location.href)
  const code = url.searchParams.get('auth_error')
  if (code === null) return null
  url.searchParams.delete('auth_error')
  window.history.replaceState(null, '', url.pathname + url.search + url.hash)
  return AUTH_ERROR_MESSAGES[code] ?? '登入失敗，請重新登入。'
}

function App() {
  const [state, setState] = useState<State>({ kind: 'loading' })
  const [authError, setAuthError] = useState<string | null>(takeAuthError)
  const [loggingOut, setLoggingOut] = useState(false)

  useEffect(() => {
    const controller = new AbortController()

    Promise.all([
      getJSON<InfoResponse>('/api/info', controller.signal),
      getJSON<HealthResponse>('/api/health', controller.signal),
      getMe(controller.signal),
    ])
      .then(([info, health, user]) => {
        setState({ kind: 'ready', name: info.name, status: health.status, user })
      })
      .catch((err: unknown) => {
        if (controller.signal.aborted) return
        console.error(err)
        setState({ kind: 'error' })
      })

    return () => controller.abort()
  }, [])

  // Login must be a full-page navigation: the backend redirects to authentik,
  // which fetch() cannot follow across origins.
  const login = () => {
    window.location.href = '/api/auth/login'
  }

  const logout = async () => {
    setLoggingOut(true)
    try {
      const res = await fetch('/api/auth/logout', { method: 'POST' })
      if (!res.ok) throw new Error(`/api/auth/logout responded with ${res.status}`)
      const body = (await res.json()) as LogoutResponse
      window.location.href = body.logout_url
    } catch (err) {
      console.error(err)
      setAuthError('登出失敗，請稍後再試。')
      setLoggingOut(false)
    }
  }

  return (
    <main className="flex min-h-screen items-center justify-center bg-blue-50 p-4">
      <div className="w-full max-w-md rounded-2xl border-t-8 border-blue-600 bg-white p-8 shadow-lg">
        {authError && (
          <div className="mb-6 rounded-lg bg-red-50 p-4 text-red-700" role="alert">
            {authError}
          </div>
        )}

        {state.kind === 'loading' && (
          <div className="flex items-center gap-3 text-slate-500" role="status">
            <span className="size-5 animate-spin rounded-full border-2 border-blue-600 border-t-transparent" />
            載入中…
          </div>
        )}

        {state.kind === 'error' && (
          <div className="rounded-lg bg-red-50 p-4 text-red-700" role="alert">
            無法連線到後端，請確認後端服務已啟動。
          </div>
        )}

        {state.kind === 'ready' && (
          <>
            <h1 className="text-3xl font-bold text-blue-700">{state.name}</h1>
            <dl className="mt-6 flex items-center justify-between rounded-lg bg-slate-50 px-4 py-3">
              <dt className="text-slate-600">後端狀態</dt>
              <dd className="flex items-center gap-2 font-mono font-semibold text-slate-800">
                <span
                  className={`size-2.5 rounded-full ${state.status === 'ok' ? 'bg-green-500' : 'bg-amber-500'}`}
                />
                {state.status}
              </dd>
            </dl>

            <section className="mt-6 border-t border-slate-200 pt-6">
              {state.user ? (
                <>
                  <p className="text-sm text-slate-500">已登入</p>
                  <p className="mt-1 text-xl font-semibold text-slate-800">{state.user.name}</p>
                  <dl className="mt-3 space-y-1 text-sm">
                    <div className="flex gap-2">
                      <dt className="w-16 text-slate-500">帳號</dt>
                      <dd className="font-mono text-slate-800">{state.user.username}</dd>
                    </div>
                    <div className="flex gap-2">
                      <dt className="w-16 text-slate-500">Email</dt>
                      <dd className="text-slate-800">{state.user.email}</dd>
                    </div>
                  </dl>
                  <button
                    type="button"
                    onClick={logout}
                    disabled={loggingOut}
                    className="mt-6 w-full rounded-lg border border-blue-600 px-4 py-2 font-medium text-blue-700 hover:bg-blue-50 disabled:opacity-50"
                  >
                    {loggingOut ? '登出中…' : '登出'}
                  </button>
                </>
              ) : (
                <>
                  <p className="text-slate-600">尚未登入</p>
                  <button
                    type="button"
                    onClick={login}
                    className="mt-4 w-full rounded-lg bg-blue-600 px-4 py-2 font-medium text-white hover:bg-blue-700"
                  >
                    登入
                  </button>
                </>
              )}
            </section>
          </>
        )}
      </div>
    </main>
  )
}

export default App
