import {StrictMode} from 'react'
import {createRoot} from 'react-dom/client'
import App from './App'
import {PWAProvider} from './features/pwa/PWAProvider'
import './styles.css'
import './finance-theme.css'
createRoot(document.getElementById('root')!).render(<StrictMode><PWAProvider><App/></PWAProvider></StrictMode>)
