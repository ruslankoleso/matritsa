import { QueryClient } from '@tanstack/react-query'
import { ApiError } from './client'

/** One cache for the whole app. It lives in a module so that sign-in and sign-out can empty it. */
export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 15_000,
      refetchOnWindowFocus: true,
      retry: (count, err) => !(err instanceof ApiError && err.status >= 400 && err.status < 500) && count < 2,
    },
  },
})
