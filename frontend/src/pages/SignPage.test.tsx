import { screen } from '@testing-library/react'
import { Route, Routes } from 'react-router-dom'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { mockApi, renderPage } from '@/test/render'
import { SignPage } from './SignPage'

afterEach(() => vi.unstubAllGlobals())

const base = { title: 'הסכם רכז/ת', body: 'סעיף 1\nסעיף 2', status: 'pending', name: 'נועה', signedAt: null, expiresAt: '2026-12-10T10:00:00Z' }
const page = (route: string) =>
  renderPage(
    <Routes>
      <Route path="/sign/:token" element={<SignPage />} />
    </Routes>,
    { route },
  )

describe('SignPage', () => {
  it('shows the contract and sends the signer to OpenSign when the contract lives there', async () => {
    mockApi({ '/public/contracts/tok': { ...base, signUrl: 'https://sign.example.com/login/abc' } })
    page('/sign/tok')
    expect(await screen.findByText('הסכם רכז/ת')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: /מעבר לחתימה/ })).toHaveAttribute('href', 'https://sign.example.com/login/abc')
    // No built-in pad: the signature happens in OpenSign.
    expect(screen.queryByRole('button', { name: 'חתימה ושליחה' })).not.toBeInTheDocument()
  })

  it('keeps the built-in signature pad when OpenSign is not used', async () => {
    mockApi({ '/public/contracts/tok': base })
    page('/sign/tok')
    expect(await screen.findByRole('button', { name: 'חתימה ושליחה' })).toBeInTheDocument()
    expect(screen.queryByRole('link', { name: /מעבר לחתימה/ })).not.toBeInTheDocument()
  })

  it('tells a signer back from OpenSign that the signature is being recorded', async () => {
    mockApi({ '/public/contracts/tok': { ...base, signUrl: 'https://sign.example.com/login/abc' } })
    page('/sign/tok?done=1')
    expect(await screen.findByText(/החתימה התקבלה/)).toBeInTheDocument()
  })

  it('confirms a signed contract', async () => {
    mockApi({ '/public/contracts/tok': { ...base, status: 'signed', signedAt: '2026-11-26T10:00:00Z' } })
    page('/sign/tok?done=1')
    expect(await screen.findByText('החוזה נחתם, תודה!')).toBeInTheDocument()
  })
})
