import { useEffect, useState } from 'react'
import { api } from '../api/client'
import { PageShell, PageInner, PageHeader, Card, Section } from '../components/ui'

export default function Identity() {
  const [providers, setProviders] = useState<{ id: string; name: string }[]>([])
  const [ldap, setLdap] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    api.identity.status().then((r) => {
      setProviders(r.providers || [])
      setLdap(!!r.ldap)
    }).catch((e: Error) => setError(e.message))
  }, [])

  return (
    <PageShell>
      <PageInner>
        <PageHeader large title="Identidade" description="OIDC para Google, Microsoft Entra, Keycloak ou um issuer genérico. LDAP e Active Directory são opcionais. O login local aceita TOTP e passkey." />
        {error && <p className="text-sm text-tone-danger mb-4">{error}</p>}
        <Section title="Provedores">
          <div className="grid sm:grid-cols-2 gap-4">
            {providers.map((p) => (
              <Card key={p.id} className="p-5">
                <div className="font-bold">{p.name}</div>
                <p className="text-sm text-text-secondary mt-2">{p.id === 'ldap' ? (ldap ? 'Configurado' : 'Defina DOCKERPILOT_LDAP_URL') : 'Issuer, client id e redirect'}</p>
              </Card>
            ))}
          </div>
        </Section>
        <Section title="Segundo fator">
          <Card className="p-5">
            <p className="text-sm text-text-secondary">TOTP de 30 segundos e passkeys WebAuthn no login local. A senha continua valendo; o fator extra só entra depois que a conta registra o segredo ou a chave.</p>
            <div className="mt-4 flex gap-2"><span className="badge-ok">TOTP</span><span className="badge-neutral">Passkey</span></div>
          </Card>
        </Section>
      </PageInner>
    </PageShell>
  )
}
