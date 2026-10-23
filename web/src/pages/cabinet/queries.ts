import { useQuery } from '@tanstack/react-query'
import { api } from '../../api/client'

export const useApplications = () => useQuery({ queryKey: ['applications'], queryFn: api.applications })
export const useApplication = (id: string) => useQuery({ queryKey: ['application', id], queryFn: () => api.application(id), enabled: !!id })
export const useTimeline = (id: string) => useQuery({ queryKey: ['timeline', id], queryFn: () => api.timeline(id), enabled: !!id })
export const useFeed = () => useQuery({ queryKey: ['feed'], queryFn: api.feed })
export const useNotifications = () => useQuery({ queryKey: ['notifications'], queryFn: api.notifications, refetchInterval: 60_000 })
export const useProducts = () => useQuery({ queryKey: ['products'], queryFn: api.products, staleTime: 5 * 60_000 })
export const useMe = () => useQuery({ queryKey: ['me'], queryFn: api.me, staleTime: 5 * 60_000 })
