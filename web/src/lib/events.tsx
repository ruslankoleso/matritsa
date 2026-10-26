import { useEffect, useLayoutEffect, useRef } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { eventsUrl } from '../api/client'
import { useAuth } from '../auth/AuthContext'

export interface LiveEvent {
  type: 'status' | 'notification' | 'queue' | 'chat'
  applicationId?: string
  threadId?: string
  status?: string
  text?: string
}

/** Opens the Server-Sent Events stream and refreshes whatever the event touches. */
export function useLiveUpdates(onEvent?: (e: LiveEvent) => void) {
  const { session } = useAuth()
  const qc = useQueryClient()
  const retry = useRef(0)
  const handler = useRef(onEvent)
  // The latest callback, without reopening the stream when it changes.
  useLayoutEffect(() => {
    handler.current = onEvent
  })

  useEffect(() => {
    if (!session) return
    let es: EventSource | null = null
    let timer: number | undefined
    let stopped = false

    const touch = (e: MessageEvent) => {
      let appId: string | undefined
      try {
        const payload = JSON.parse(e.data) as LiveEvent
        appId = payload.applicationId
        handler.current?.(payload)
      } catch {
        /* ignore malformed event */
      }
      void qc.invalidateQueries({ queryKey: ['applications'] })
      void qc.invalidateQueries({ queryKey: ['feed'] })
      void qc.invalidateQueries({ queryKey: ['notifications'] })
      void qc.invalidateQueries({ queryKey: ['queue'] })
      void qc.invalidateQueries({ queryKey: ['chat'] })
      void qc.invalidateQueries({ queryKey: ['support'] })
      if (appId) {
        void qc.invalidateQueries({ queryKey: ['application', appId] })
        void qc.invalidateQueries({ queryKey: ['timeline', appId] })
        void qc.invalidateQueries({ queryKey: ['staffApp', appId] })
      }
    }

    const retryLater = () => {
      const wait = Math.min(15_000, 1000 * 2 ** retry.current++)
      timer = window.setTimeout(() => void connect(), wait)
    }

    // Every connection gets a fresh ticket: the request for it also refreshes an expired access token.
    const connect = async () => {
      if (stopped) return
      let url: string
      try {
        url = await eventsUrl()
      } catch {
        retryLater()
        return
      }
      if (stopped) return
      es = new EventSource(url)
      es.onopen = () => {
        retry.current = 0
      }
      ['status', 'notification', 'queue', 'chat'].forEach((t) => es!.addEventListener(t, touch as EventListener))
      es.onerror = () => {
        es?.close()
        retryLater()
      }
    }
    void connect()
    return () => {
      stopped = true
      window.clearTimeout(timer)
      es?.close()
    }
  }, [session, qc])
}
