import { create } from 'zustand'
import { persist } from 'zustand/middleware'
import type { User, Workspace, Organization } from '@/types'

interface AuthState {
  token: string | null
  user: User | null
  org: Organization | null
  workspace: Workspace | null
  workspaces: Workspace[]
  role: string
  setAuth: (data: {
    token?: string
    user: User | null
    org?: Organization | null
    workspace: Workspace | null
    workspaces: Workspace[]
    role?: string
  }) => void
  setWorkspace: (ws: Workspace | null) => void
  clear: () => void
}

export const useAuthStore = create<AuthState>()(
  persist(
    (set) => ({
      token: null,
      user: null,
      org: null,
      workspace: null,
      workspaces: [],
      role: '',
      setAuth: (data) =>
        set((state) => ({
          token: data.token ?? state.token,
          user: data.user,
          org: data.org ?? state.org,
          workspace: data.workspace,
          workspaces: data.workspaces,
          role: data.role ?? state.role
        })),
      setWorkspace: (ws) => set({ workspace: ws }),
      clear: () =>
        set({
          token: null,
          user: null,
          org: null,
          workspace: null,
          workspaces: [],
          role: ''
        })
    }),
    {
      name: 'oah-auth',
      partialize: (state) => ({
        token: state.token,
        user: state.user,
        org: state.org,
        workspace: state.workspace,
        workspaces: state.workspaces,
        role: state.role
      })
    }
  )
)
