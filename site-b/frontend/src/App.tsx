import { useEffect, useState } from 'react'

type InfoResponse = { name: string }
type HealthResponse = { status: string }

type State =
  | { kind: 'loading' }
  | { kind: 'error' }
  | { kind: 'ready'; name: string; status: string }

async function getJSON<T>(path: string, signal: AbortSignal): Promise<T> {
  const res = await fetch(path, { signal })
  if (!res.ok) {
    throw new Error(`${path} responded with ${res.status}`)
  }
  return (await res.json()) as T
}

function App() {
  const [state, setState] = useState<State>({ kind: 'loading' })

  useEffect(() => {
    const controller = new AbortController()

    Promise.all([
      getJSON<InfoResponse>('/api/info', controller.signal),
      getJSON<HealthResponse>('/api/health', controller.signal),
    ])
      .then(([info, health]) => {
        setState({ kind: 'ready', name: info.name, status: health.status })
      })
      .catch((err: unknown) => {
        if (controller.signal.aborted) return
        console.error(err)
        setState({ kind: 'error' })
      })

    return () => controller.abort()
  }, [])

  return (
    <main className="flex min-h-screen items-center justify-center bg-emerald-50 p-4">
      <div className="w-full max-w-md rounded-2xl border-t-8 border-emerald-600 bg-white p-8 shadow-lg">
        {state.kind === 'loading' && (
          <div className="flex items-center gap-3 text-slate-500" role="status">
            <span className="size-5 animate-spin rounded-full border-2 border-emerald-600 border-t-transparent" />
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
            <h1 className="text-3xl font-bold text-emerald-700">{state.name}</h1>
            <dl className="mt-6 flex items-center justify-between rounded-lg bg-slate-50 px-4 py-3">
              <dt className="text-slate-600">後端狀態</dt>
              <dd className="flex items-center gap-2 font-mono font-semibold text-slate-800">
                <span
                  className={`size-2.5 rounded-full ${state.status === 'ok' ? 'bg-green-500' : 'bg-amber-500'}`}
                />
                {state.status}
              </dd>
            </dl>
          </>
        )}
      </div>
    </main>
  )
}

export default App
