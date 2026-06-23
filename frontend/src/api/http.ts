import axios, { type AxiosInstance, type InternalAxiosRequestConfig } from 'axios'
import { message } from 'antd'

const http: AxiosInstance = axios.create({
  baseURL: '/api',
  timeout: 30000,
  headers: {
    'Content-Type': 'application/json'
  }
})

http.interceptors.request.use((config: InternalAxiosRequestConfig) => {
  const token = localStorage.getItem('oah_token')
  if (token && config.headers) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

http.interceptors.response.use(
  (resp) => {
    const data = resp.data
    if (data && typeof data === 'object' && 'code' in data) {
      if (data.code === 0) {
        // Unwrap first layer {code, message, data}
        const inner = data.data
        // Smart unwrap: some endpoints return paginated wrapper {items, page, page_size, total}
        // In that case return items array directly to the caller
        if (inner && typeof inner === 'object' && Array.isArray(inner.items) && !('code' in inner)) {
          return inner.items
        }
        return inner
      }
      message.error(data.message || 'Request failed')
      return Promise.reject(new Error(data.message || 'Request failed'))
    }
    return data
  },
  (err) => {
    const status = err?.response?.status
    const respData = err?.response?.data
    const msg = respData?.message || err.message || 'Network error'
    if (status === 401) {
      localStorage.removeItem('oah_token')
      message.error('Session expired, please log in again')
      setTimeout(() => {
        if (window.location.pathname !== '/login') {
          window.location.href = '/login'
        }
      }, 500)
    } else {
      message.error(msg)
    }
    return Promise.reject(err)
  }
)

export default http
