import i18next from 'i18next';
import { initReactI18next } from 'react-i18next';
import LanguageDetector from 'i18next-browser-languagedetector';

function init() {
  i18next
    .use(LanguageDetector)
    .use(initReactI18next)
    .init({
      resources: {},
      defaultNS: 'defNS',
      fallbackLng: 'en',
      interpolation: {
        escapeValue: false,
      },
    });
}

function addTranslations(res) {
  for (const lng in res) {
    const lngIETF = lng.replace(/_/g, '-')
    i18next.addResourceBundle(lngIETF, "defNS", res[lng], true, false)
  }
}

function t(id) {
  return i18next.t(id)
}


export default {
  init,
  addTranslations,
  t
};
