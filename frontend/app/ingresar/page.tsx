import type { Metadata } from 'next';

import { AuthExperience } from '../components/auth-experience';

export const metadata: Metadata = {
  title: 'Ingresar | Hausy',
};

export default function SignInPage() {
  return <AuthExperience />;
}
