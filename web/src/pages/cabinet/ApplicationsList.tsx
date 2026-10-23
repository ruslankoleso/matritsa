import { LinkButton } from '../../design/Button'
import { Empty, ErrorBox, Loading } from '../../design/states'
import { useApplications } from './queries'
import { ApplicationsAccordion } from './parts'

export default function ApplicationsList() {
  const apps = useApplications()
  return (
    <>
      <div className="page-head">
        <div>
          <h1>Мои заявки</h1>
          {apps.data && <p className="sub">Всего: {apps.data.length}</p>}
        </div>
        <LinkButton to="/cabinet/new">Новая заявка</LinkButton>
      </div>
      <section className="card">
        {apps.isLoading ? <Loading /> : apps.isError ? <ErrorBox error={apps.error} onRetry={() => void apps.refetch()} /> : apps.data && apps.data.length ? (
          <ApplicationsAccordion items={apps.data} />
        ) : (
          <Empty title="Заявок пока нет">Начните с новой заявки — это займёт несколько минут.</Empty>
        )}
      </section>
    </>
  )
}
