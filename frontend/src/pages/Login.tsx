import { useState } from 'react'
import { Form, Input, Button, Card, Typography, Tabs, App, Space, Dropdown } from 'antd'
import { LockOutlined, UserOutlined, GlobalOutlined } from '@ant-design/icons'
import { useNavigate } from 'react-router-dom'
import { authApi } from '@/api'
import { useAuthStore } from '@/store/auth'
import { useTranslation } from 'react-i18next'

const { Title, Text } = Typography

type Tab = 'login' | 'register'

const LoginPage = () => {
  const { t, i18n } = useTranslation()
  const navigate = useNavigate()
  const setAuth = useAuthStore((s) => s.setAuth)
  const { message } = App.useApp()
  const [tab, setTab] = useState<Tab>('login')
  const [loading, setLoading] = useState(false)

  const onLogin = async (values: { username: string; password: string }) => {
    setLoading(true)
    try {
      const data = await authApi.login(values.username, values.password)
      localStorage.setItem('oah_token', data.token)
      setAuth({
        token: data.token,
        user: data.user,
        org: data.org,
        workspace: data.workspace,
        workspaces: data.workspaces,
        role: data.role
      })
      message.success(t('login.welcomeBack', { name: data.user.display_name || data.user.username }))
      navigate('/dashboard')
    } catch {
      /* interceptor */
    } finally {
      setLoading(false)
    }
  }

  const onRegister = async (values: { username: string; display_name: string; password: string }) => {
    setLoading(true)
    try {
      const data = await authApi.register({
        username: values.username,
        display_name: values.display_name,
        password: values.password
      })
      localStorage.setItem('oah_token', data.token)
      setAuth({
        token: data.token,
        user: data.user,
        org: data.org,
        workspace: data.workspace,
        workspaces: data.workspaces,
        role: data.role
      })
      message.success(t('login.registerSuccess'))
      navigate('/dashboard')
    } catch {
      /* */
    } finally {
      setLoading(false)
    }
  }

  return (
    <div
      style={{
        minHeight: '100vh',
        background: 'linear-gradient(135deg, #1677ff 0%, #722ed1 100%)',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        padding: 24,
        position: 'relative'
      }}
    >
      <div style={{ position: 'absolute', top: 24, right: 24 }}>
        <Dropdown
          menu={{
            items: [
              { key: 'zh', label: '简体中文' },
              { key: 'en', label: 'English' }
            ],
            onClick: ({ key }) => {
              i18n.changeLanguage(key)
            }
          }}
        >
          <Button
            style={{
              background: 'rgba(255, 255, 255, 0.15)',
              borderColor: 'rgba(255, 255, 255, 0.25)',
              color: '#fff',
              backdropFilter: 'blur(8px)',
              boxShadow: '0 4px 12px rgba(0,0,0,0.1)'
            }}
            icon={<GlobalOutlined />}
          >
            {i18n.language.startsWith('zh') ? '中文' : 'English'}
          </Button>
        </Dropdown>
      </div>
      <Card style={{ width: 440, boxShadow: '0 10px 30px rgba(0,0,0,0.2)' }}>

        <Space direction="vertical" size="middle" style={{ width: '100%' }}>
          <div style={{ textAlign: 'center' }}>
            <Title level={3} style={{ marginBottom: 4 }}>
              🪐 Open Agent Hub
            </Title>
            <Text type="secondary">{t('login.subtitle')}</Text>
          </div>
          <Tabs
            activeKey={tab}
            onChange={(k) => setTab(k as Tab)}
            centered
            items={[
              {
                key: 'login',
                label: t('login.login'),
                children: (
                  <Form
                    layout="vertical"
                    onFinish={onLogin}
                    initialValues={{ username: 'admin', password: 'admin123' }}
                  >
                    <Form.Item
                      name="username"
                      label={t('login.username')}
                      rules={[
                        { required: true, message: t('login.usernameRequired') },
                        { min: 2, message: t('login.usernameInvalid') },
                        { pattern: /^[a-zA-Z\u4e00-\u9fff][a-zA-Z0-9\u4e00-\u9fff]*$/, message: t('login.usernameInvalid') }
                      ]}
                    >
                      <Input prefix={<UserOutlined />} placeholder={t('login.usernamePlaceholder')} />
                    </Form.Item>
                    <Form.Item
                      name="password"
                      label={t('login.password')}
                      rules={[{ required: true, message: t('login.passwordRequired') }]}
                    >
                      <Input.Password prefix={<LockOutlined />} placeholder={t('login.password')} />
                    </Form.Item>
                    <Form.Item>
                      <Button type="primary" htmlType="submit" block loading={loading}>
                        {t('login.login')}
                      </Button>
                    </Form.Item>
                    <Text type="secondary" style={{ fontSize: 12 }}>
                      {t('login.demoAccountHint')}<code>admin</code> / <code>admin123</code>
                    </Text>
                  </Form>
                )
              },
              {
                key: 'register',
                label: t('login.register'),
                children: (
                  <Form layout="vertical" onFinish={onRegister}>
                    <Form.Item
                      name="username"
                      label={t('login.username')}
                      rules={[
                        { required: true, message: t('login.usernameRequired') },
                        { min: 2, message: t('login.usernameInvalid') },
                        { pattern: /^[a-zA-Z\u4e00-\u9fff][a-zA-Z0-9\u4e00-\u9fff]*$/, message: t('login.usernameInvalid') }
                      ]}
                    >
                      <Input prefix={<UserOutlined />} placeholder={t('login.usernamePlaceholder')} />
                    </Form.Item>
                    <Form.Item
                      name="display_name"
                      label={t('login.displayName')}
                      rules={[{ required: true, message: t('login.displayNameRequired') }]}
                    >
                      <Input prefix={<UserOutlined />} placeholder={t('login.displayNamePlaceholder')} />
                    </Form.Item>
                    <Form.Item
                      name="password"
                      label={t('login.password')}
                      rules={[
                        { required: true, message: t('login.passwordRequired') },
                        { min: 6, message: t('login.passwordMinLength') }
                      ]}
                    >
                      <Input.Password prefix={<LockOutlined />} placeholder={t('login.password')} />
                    </Form.Item>
                    <Form.Item>
                      <Button type="primary" htmlType="submit" block loading={loading}>
                        {t('login.registerAndCreate')}
                      </Button>
                    </Form.Item>
                  </Form>
                )
              }
            ]}
          />
        </Space>
      </Card>
    </div>
  )
}

export default LoginPage
