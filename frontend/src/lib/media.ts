import { useEffect, useState } from 'react'

const PHONE = '(max-width: 639px)'

/** True below Tailwind's `sm` breakpoint (phones). Follows rotation / resizing. */
export function useIsPhone() {
  const [phone, setPhone] = useState(() => typeof window !== 'undefined' && window.matchMedia(PHONE).matches)
  useEffect(() => {
    const m = window.matchMedia(PHONE)
    const on = () => setPhone(m.matches)
    m.addEventListener('change', on)
    return () => m.removeEventListener('change', on)
  }, [])
  return phone
}
