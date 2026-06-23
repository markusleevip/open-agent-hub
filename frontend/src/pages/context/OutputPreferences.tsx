import { useEffect, useMemo, useState } from 'react'
import { Alert, App, Button, Card, Col, Form, Input, Row, Select, Space, Switch, Typography } from 'antd'
import { SaveOutlined } from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import { personalInstructionsApi } from '@/api'
import type { PersonalInstructions } from '@/types'

const { Title, Text } = Typography
const { TextArea } = Input

const defaultValues: PersonalInstructions = {
  language: 'zh-CN',
  verbosity: 'normal',
  code_style: 'google',
  personality: 'pragmatic',
  response_style: 'direct',
  custom_instructions: '',
  memory: {
    enabled: true,
    skip_tool_context: true
  }
}

const OutputPreferences = () => {
  const { t } = useTranslation()
  const { message } = App.useApp()
  const [form] = Form.useForm<PersonalInstructions>()
  const [loading, setLoading] = useState(false)
  const [saving, setSaving] = useState(false)
  const [dirty, setDirty] = useState(false)

  const languageOptions = useMemo(
    () => [
      { value: 'zh-CN', label: t('context.personal.languageZh') },
      { value: 'en-US', label: t('context.personal.languageEn') },
      { value: 'auto', label: t('context.personal.languageAuto') }
    ],
    [t]
  )

  const verbosityOptions = useMemo(
    () => [
      { value: 'concise', label: t('context.personal.verbosityConcise') },
      { value: 'normal', label: t('context.personal.verbosityNormal') },
      { value: 'detailed', label: t('context.personal.verbosityDetailed') }
    ],
    [t]
  )

  const codeStyleOptions = useMemo(
    () => [
      { value: 'google', label: t('context.personal.codeStyleGoogle') },
      { value: 'standard', label: t('context.personal.codeStyleStandard') },
      { value: 'project', label: t('context.personal.codeStyleProject') },
      { value: 'custom', label: t('context.personal.codeStyleCustom') }
    ],
    [t]
  )

  const personalityOptions = useMemo(
    () => [
      { value: 'pragmatic', label: t('context.personal.personalityPragmatic') },
      { value: 'concise', label: t('context.personal.personalityConcise') },
      { value: 'rigorous', label: t('context.personal.personalityRigorous') },
      { value: 'friendly', label: t('context.personal.personalityFriendly') },
      { value: 'custom', label: t('context.personal.personalityCustom') }
    ],
    [t]
  )

  const responseStyleOptions = useMemo(
    () => [
      { value: 'direct', label: t('context.personal.responseDirect') },
      { value: 'explanatory', label: t('context.personal.responseExplanatory') },
      { value: 'checklist', label: t('context.personal.responseChecklist') }
    ],
    [t]
  )

  useEffect(() => {
    load()
  }, [])

  const load = async () => {
    setLoading(true)
    try {
      const data = await personalInstructionsApi.get()
      form.setFieldsValue({ ...defaultValues, ...data, memory: { ...defaultValues.memory, ...data.memory } })
      setDirty(false)
    } finally {
      setLoading(false)
    }
  }

  const onSave = async () => {
    const values = await form.validateFields()
    setSaving(true)
    try {
      const saved = await personalInstructionsApi.update({
        ...defaultValues,
        ...values,
        memory: { ...defaultValues.memory, ...values.memory }
      })
      form.setFieldsValue(saved)
      setDirty(false)
      message.success(t('context.personal.saved'))
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="page-container">
      <Card
        title={<Title level={4} style={{ margin: 0 }}>{t('context.personal.title')}</Title>}
        loading={loading}
        extra={
          <Button
            type="primary"
            icon={<SaveOutlined />}
            onClick={onSave}
            loading={saving}
            disabled={!dirty}
          >
            {t('context.personal.save')}
          </Button>
        }
      >
        <Space direction="vertical" size={18} style={{ width: '100%' }}>
          <Text type="secondary">{t('context.personal.description')}</Text>
          <Alert type="info" showIcon message={t('context.personal.priorityHint')} />
          <Alert
            type="success"
            showIcon
            message={t('context.personal.agentApplyTitle')}
            description={
              <Space direction="vertical" size={4}>
                <Text>{t('context.personal.agentApplyDesc')}</Text>
                <Text code>hub.sync_project</Text>
                <Text type="secondary">{t('context.personal.agentApplyPath')}</Text>
              </Space>
            }
          />

          <Form
            form={form}
            layout="vertical"
            initialValues={defaultValues}
            onValuesChange={() => setDirty(true)}
          >
            <Row gutter={16}>
              <Col xs={24} md={12} xl={8}>
                <Form.Item name="personality" label={t('context.personal.personality')} rules={[{ required: true }]}>
                  <Select options={personalityOptions} />
                </Form.Item>
              </Col>
              <Col xs={24} md={12} xl={8}>
                <Form.Item name="language" label={t('context.personal.language')} rules={[{ required: true }]}>
                  <Select options={languageOptions} />
                </Form.Item>
              </Col>
              <Col xs={24} md={12} xl={8}>
                <Form.Item name="verbosity" label={t('context.personal.verbosity')} rules={[{ required: true }]}>
                  <Select options={verbosityOptions} />
                </Form.Item>
              </Col>
              <Col xs={24} md={12} xl={8}>
                <Form.Item name="response_style" label={t('context.personal.responseStyle')} rules={[{ required: true }]}>
                  <Select options={responseStyleOptions} />
                </Form.Item>
              </Col>
              <Col xs={24} md={12} xl={8}>
                <Form.Item name="code_style" label={t('context.personal.codeStyle')} rules={[{ required: true }]}>
                  <Select options={codeStyleOptions} />
                </Form.Item>
              </Col>
            </Row>

            <Form.Item
              name="custom_instructions"
              label={t('context.personal.customInstructions')}
              rules={[{ max: 20000, message: t('context.personal.customTooLong') }]}
            >
              <TextArea rows={12} showCount maxLength={20000} placeholder={t('context.personal.customPlaceholder')} />
            </Form.Item>

            <Row gutter={16}>
              <Col xs={24} md={12}>
                <Form.Item
                  name={['memory', 'enabled']}
                  label={t('context.personal.memoryEnabled')}
                  valuePropName="checked"
                >
                  <Switch />
                </Form.Item>
              </Col>
              <Col xs={24} md={12}>
                <Form.Item
                  name={['memory', 'skip_tool_context']}
                  label={t('context.personal.skipToolContext')}
                  valuePropName="checked"
                >
                  <Switch />
                </Form.Item>
              </Col>
            </Row>
          </Form>
        </Space>
      </Card>
    </div>
  )
}

export default OutputPreferences
