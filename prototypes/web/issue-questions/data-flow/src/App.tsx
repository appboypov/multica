import { Theme } from './settings/types';
import { UJ1DataFlowPrototype } from './components/generated/UJ1DataFlowPrototype';

let theme: Theme = 'light';

function App() {
  function setTheme(theme: Theme) {
    if (theme === 'dark') {
      document.documentElement.classList.add('dark');
    } else {
      document.documentElement.classList.remove('dark');
    }
  }

  setTheme(theme);

  return (
    <>
      <UJ1DataFlowPrototype />
    </>
  ); // %EXPORT_STATEMENT%
}

export default App;
