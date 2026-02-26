import { useEffect, useRef } from "react";
import { useLocation, useRoute } from "wouter";
import { APP_ROUTES, buildArticleRoute, parseRouteArticleID } from "@/lib/router";

export function useArticleRoute(selectedArticleID: number | null, onRouteSelect: (id: number) => void) {
  const [, setLocation] = useLocation();
  const [matched, params] = useRoute(APP_ROUTES.article);
  const onRouteSelectRef = useRef(onRouteSelect);

  useEffect(() => {
    onRouteSelectRef.current = onRouteSelect;
  }, [onRouteSelect]);

  useEffect(() => {
    if (!matched) {
      return;
    }
    const routeID = parseRouteArticleID(params?.id);
    if (routeID == null) {
      return;
    }
    if (routeID !== selectedArticleID) {
      onRouteSelectRef.current(routeID);
    }
  }, [matched, params?.id, selectedArticleID]);

  const pushArticleRoute = (id: number) => {
    setLocation(buildArticleRoute(id), { replace: false });
  };

  const clearArticleRoute = () => {
    setLocation(APP_ROUTES.home, { replace: false });
  };

  return { pushArticleRoute, clearArticleRoute };
}
