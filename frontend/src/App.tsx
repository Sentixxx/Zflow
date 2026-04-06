import { Route, Switch } from "wouter";
import { ReaderPage, BriefsPage } from "@/pages";

export function App() {
  return (
    <Switch>
      <Route path="/">{() => <ReaderPage />}</Route>
      <Route path="/settings">{() => <ReaderPage initialSettingsOpen={true} />}</Route>
      <Route path="/briefs">{() => <BriefsPage />}</Route>
      <Route>{() => <ReaderPage />}</Route>
    </Switch>
  );
}
  
