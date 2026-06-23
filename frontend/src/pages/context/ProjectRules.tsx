import RuleManager from '@/components/RuleManager'
import { useTranslation } from 'react-i18next'

const ProjectRules = () => {
  const { t } = useTranslation()
  return (
    <RuleManager
      title={t('context.projectRulesTitle')}
      description={t('context.projectRulesDesc')}
      scope="project"
      fixedType="project_rule"
      withProject
    />
  )
}

export default ProjectRules
