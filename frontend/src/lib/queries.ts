import { useQuery } from '@tanstack/react-query'
import { api } from './api'
import type { Activity, Building, Customer, EquipmentItem, Room, User } from './types'

const long = { staleTime: 5 * 60_000 }

export const useBuildings = () => useQuery({ queryKey: ['buildings'], queryFn: () => api.get<Building[]>('/buildings'), ...long })
export const useRooms = () => useQuery({ queryKey: ['rooms'], queryFn: () => api.get<Room[]>('/rooms'), ...long })
export const useEquipment = () => useQuery({ queryKey: ['equipment'], queryFn: () => api.get<EquipmentItem[]>('/equipment'), ...long })
export const useActivities = (enabled = true) =>
  useQuery({ queryKey: ['activities'], queryFn: () => api.get<Activity[]>('/activities'), enabled, ...long })
export const useUsers = (role?: string, enabled = true) =>
  useQuery({ queryKey: ['users', role ?? 'all'], queryFn: () => api.get<User[]>('/users', { role }), enabled, ...long })
export const useCustomers = (q = '', enabled = true) =>
  useQuery({ queryKey: ['customers', q], queryFn: () => api.get<Customer[]>('/customers', { q }), enabled })

/** Lookup helpers built from the reference lists. */
export function useLookups() {
  const rooms = useRooms().data ?? []
  const buildings = useBuildings().data ?? []
  const equipment = useEquipment().data ?? []
  return {
    rooms,
    buildings,
    equipment,
    roomById: new Map(rooms.map((r) => [r.id, r])),
    buildingById: new Map(buildings.map((b) => [b.id, b])),
    equipmentLabel: (code: string) => equipment.find((e) => e.code === code)?.label ?? code,
  }
}
