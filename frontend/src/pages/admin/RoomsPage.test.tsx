import { screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { mockApi, renderPage } from '@/test/render'
import { RoomsPage } from './RoomsPage'

afterEach(() => vi.unstubAllGlobals())

const building = { id: 'b-1', name: 'המרכז הקהילתי', address: 'רמות', opensAt: '07:00', closesAt: '23:00', isActive: true }

describe('RoomsPage', () => {
  it('guides the admin to create a building on a brand-new instance instead of loading forever', async () => {
    mockApi({ '/rooms': [], '/buildings': [], '/equipment': [], '/bookings': [] })
    renderPage(<RoomsPage />)
    expect(await screen.findByText('עדיין לא הוגדר מבנה')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: /הגדרות מערכת/ })).toHaveAttribute('href', '/admin/settings')
  })

  it('offers to add the first room once a building exists', async () => {
    mockApi({ '/rooms': [], '/buildings': [building], '/equipment': [], '/bookings': [] })
    renderPage(<RoomsPage />)
    expect(await screen.findByText('עדיין אין חדרים')).toBeInTheDocument()
    expect(screen.getAllByRole('button', { name: /חדר חדש/ }).length).toBeGreaterThan(0)
  })

  it('lists rooms by building', async () => {
    const room = { id: 'r-1', buildingId: 'b-1', name: 'חדר תפירה', kind: 'classroom', capacity: 20, color: '#0369a1', hourlyRate: 90, minHours: 1,
      bufferMinutes: 10, isBookable: true, isActive: true, sortOrder: 1, equipment: [], description: '', rules: '' }
    mockApi({ '/rooms': [room], '/buildings': [building], '/equipment': [], '/bookings': [] })
    renderPage(<RoomsPage />)
    expect(await screen.findByText('חדר תפירה')).toBeInTheDocument()
    expect(screen.queryByText('עדיין אין חדרים')).not.toBeInTheDocument()
  })
})
