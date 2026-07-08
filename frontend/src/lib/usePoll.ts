import { useEffect, useRef, type DependencyList } from 'react'
import { POLL_INTERVAL_MS } from './constants'

/** Executa `fn` na montagem e a cada `intervalMs` (padrão 15s). */
export function usePoll(fn: () => void, deps: DependencyList, intervalMs = POLL_INTERVAL_MS) {
  const ref = useRef(fn)
  ref.current = fn

  useEffect(() => {
    const run = () => ref.current()
    run()
    const id = setInterval(run, intervalMs)
    return () => clearInterval(id)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, deps)
}
