import React from 'react'
import ReactDOM from 'react-dom/client'
import { RouterProvider } from 'react-router-dom'
import { ConfigProvider, App as AntdApp } from 'antd'
import zhCN from 'antd/locale/zh_CN'
import enUS from 'antd/locale/en_US'
import 'antd/dist/reset.css'
import './index.css'
import { router } from './router'
import './i18n'
import { useTranslation } from 'react-i18next'

const AppWrapper = () => {
  const { i18n } = useTranslation()
  const antdLocale = i18n.language.startsWith('zh') ? zhCN : enUS

  return (
    <ConfigProvider
      locale={antdLocale}
      theme={{
        token: {
          colorPrimary: '#1677ff',
          borderRadius: 6
        }
      }}
    >
      <AntdApp>
        <RouterProvider router={router} />
      </AntdApp>
    </ConfigProvider>
  )
}

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <AppWrapper />
  </React.StrictMode>
)

