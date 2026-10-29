import { useEffect, useRef, useState } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { getToken } from './api'
import { useAuth } from './auth'

export interface RealtimeEvent {
  type: string
  data?: unknown
  at: number
}

type Listener = (ev: RealtimeEvent) => void

/**
 * Opens a self-healing WebSocket (exponential backoff) and calls onEvent for
 * each message. Returns whether the socket is currently connected.
 */
export function useSocket(path: '/ws' | '/ws/signage', onEvent: Listener, enabled = true) {
  const [connected, setConnected] = useState(false)
  const handler = useRef(onEvent)
  handler.current = onEvent

  useEffect(() => {
    if (!enabled) return
    let ws: WebSocket | null = null
    let retry = 0
    let timer: ReturnType<typeof setTimeout> | undefined
    let closed = false

    const connect = () => {
      const proto = location.protocol === 'https:' ? 'wss' : 'ws'
      const token = path === '/ws' ? getToken() : null
      const url = `${proto}://${location.host}${path}${token ? `?token=${encodeURIComponent(token)}` : ''}`
      ws = new WebSocket(url)
      ws.onopen = () => {
        retry = 0
        setConnected(true)
      }
      ws.onmessage = (m) => {
        try {
          handler.current(JSON.parse(m.data))
        } catch {
          /* ignore malformed */
        }
      }
      ws.onclose = () => {
        setConnected(false)
        if (closed) return
        retry = Math.min(retry + 1, 6)
        timer = setTimeout(connect, 500 * 2 ** retry)
      }
    }
    connect()
    return () => {
      closed = true
      clearTimeout(timer)
      ws?.close()
    }
  }, [path, enabled])

  return connected
}

/** Maps realtime events to react-query cache invalidations for the staff app. */
const invalidations: Record<string, string[][]> = {
  'booking.changed': [['bookings'], ['dashboard'], ['availability'], ['balances'], ['reports']],
  'request.changed': [['requests'], ['dashboard']],
  'payment.changed': [['payments'], ['balances'], ['dashboard'], ['bookings']],
  'task.changed': [['tasks'], ['dashboard']],
  notification: [['notifications']],
}

export function useLiveInvalidation(enabled: boolean) {
  const qc = useQueryClient()
  return useSocket(
    '/ws',
    (ev) => {
      for (const key of invalidations[ev.type] || []) qc.invalidateQueries({ queryKey: key })
      if (ev.type === 'organization.changed') useAuth.getState().reloadConfig()
    },
    enabled,
  )
}
