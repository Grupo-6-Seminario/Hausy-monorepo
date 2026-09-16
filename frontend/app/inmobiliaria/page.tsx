import type { Metadata } from 'next';

import { AgencyDashboard } from '../components/agency-dashboard';

export const metadata: Metadata = {
  title: 'Catálogo inmobiliario | Hausy',
};

export default function AgencyPage() {
  return <AgencyDashboard />;
}
