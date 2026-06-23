import RuleManager from '@/components/RuleManager'
import { useTranslation } from 'react-i18next'

const GlobalRules = () => {
  const { t } = useTranslation()
  return (
    <RuleManager
      title={t('context.globalRulesTitle')}
      description={t('context.globalRulesDesc')}
      scope="workspace"
      fixedType="global_rule"
    />
  )
}

export default GlobalRules
