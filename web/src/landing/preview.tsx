import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import '../fonts';
import '../styles/index.css';
import { Landing } from './Landing';

// Dev-only: renders the landing page without a backend so inspect.mjs can measure it.
createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <Landing />
  </StrictMode>,
);
