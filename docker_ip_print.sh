for container_id in $(docker ps -q); do
  container_name=$(docker inspect -f '{{.Name}}' $container_id | sed 's/^\///')
  container_ip=$(docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}},{{end}}' $container_id)
  echo "${container_name}: ${container_ip}"
done
